package server

import (
	"context"
	"errors"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

// projectArchiveGate reports whether the named project blocks a lifecycle action.
// It returns CodeProjectArchived (using the supplied message) when the project is
// archived, and CodeInternal when its definition is unreadable for any reason
// other than absence — corrupt or I/O/permission failures fail closed because the
// archived state is unknown and the file may itself record archived: true. A
// missing definition (ErrNotFound) passes, matching catalog omission
// (FS-05.R34, TS-03.R20).
func (s *Server) projectArchiveGate(projectID, archivedMsg string) *runtime.APIError {
	project, err := s.configStore.ReadProject(projectID)
	switch {
	case err == nil:
		if project.Archived {
			return apiError(runtime.CodeProjectArchived, archivedMsg)
		}
		return nil
	case errors.Is(err, config.ErrNotFound):
		return nil
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}

// acquireProjectStart is the shared launch boundary. It deliberately spans the
// final registry registration so a project archive cannot stop a snapshot of
// agents and then race a new process into the durable archive commit.
func (s *Server) acquireProjectStart(project string) *runtime.APIError {
	s.archiveMu.Lock()
	defer s.archiveMu.Unlock()
	if s.projectArchiving[project] {
		return apiError(runtime.CodeProjectArchiving, "project archival is in progress")
	}
	s.projectStartLeases[project]++
	return nil
}

// acquireAgentStart joins a process-start path to both project and agent
// transition gates. Resume and switch use it so an individual archive claim
// cannot commit beside a newly registered runtime.
func (s *Server) acquireAgentStart(project, agent string) *runtime.APIError {
	s.archiveMu.Lock()
	defer s.archiveMu.Unlock()
	if s.projectArchiving[project] {
		return apiError(runtime.CodeProjectArchiving, "project archival is in progress")
	}
	if s.agentArchiving[agent] {
		return apiError(runtime.CodeAgentArchiving, "agent archival is in progress")
	}
	s.projectStartLeases[project]++
	s.agentStartLeases[agent]++
	return nil
}

func (s *Server) releaseProjectStart(project string) {
	s.archiveMu.Lock()
	defer s.archiveMu.Unlock()
	if s.projectStartLeases[project] <= 1 {
		delete(s.projectStartLeases, project)
	} else {
		s.projectStartLeases[project]--
	}
	s.signalArchiveChangedLocked()
}

func (s *Server) releaseAgentStart(project, agent string) {
	s.archiveMu.Lock()
	defer s.archiveMu.Unlock()
	if s.projectStartLeases[project] <= 1 {
		delete(s.projectStartLeases, project)
	} else {
		s.projectStartLeases[project]--
	}
	if s.agentStartLeases[agent] <= 1 {
		delete(s.agentStartLeases, agent)
	} else {
		s.agentStartLeases[agent]--
	}
	s.signalArchiveChangedLocked()
}

// beginProjectArchive reserves its exclusive claim before waiting for older
// starts or agent transitions. Reserving first prevents later starts from
// leapfrogging an archive request.
func (s *Server) beginProjectArchive(ctx context.Context, project string) *runtime.APIError {
	s.archiveMu.Lock()
	if s.projectArchiving[project] {
		s.archiveMu.Unlock()
		return apiError(runtime.CodeProjectArchiving, "project archival is in progress")
	}
	s.projectArchiving[project] = true
	for s.projectStartLeases[project] != 0 || s.projectTransitions[project] != 0 {
		changed := s.archiveChanged
		s.archiveMu.Unlock()
		select {
		case <-ctx.Done():
			s.archiveMu.Lock()
			delete(s.projectArchiving, project)
			s.signalArchiveChangedLocked()
			s.archiveMu.Unlock()
			return apiError(runtime.CodeConflict, "project archival was cancelled")
		case <-changed:
		}
		s.archiveMu.Lock()
	}
	s.archiveMu.Unlock()
	return nil
}

// beginAgentArchive blocks Resume/Switch for one agent and participates in the
// project archive wait, so Restore cannot clear a flag after a project archive
// has taken its snapshot.
func (s *Server) beginAgentArchive(ctx context.Context, project, agent string) *runtime.APIError {
	s.archiveMu.Lock()
	if s.projectArchiving[project] {
		s.archiveMu.Unlock()
		return apiError(runtime.CodeProjectArchiving, "project archival is in progress")
	}
	if s.agentArchiving[agent] {
		s.archiveMu.Unlock()
		return apiError(runtime.CodeAgentArchiving, "agent archival is in progress")
	}
	s.agentArchiving[agent] = true
	s.projectTransitions[project]++
	for s.agentStartLeases[agent] != 0 {
		changed := s.archiveChanged
		s.archiveMu.Unlock()
		select {
		case <-ctx.Done():
			s.archiveMu.Lock()
			s.endAgentArchiveLocked(project, agent)
			s.archiveMu.Unlock()
			return apiError(runtime.CodeConflict, "agent archival was cancelled")
		case <-changed:
		}
		s.archiveMu.Lock()
	}
	s.archiveMu.Unlock()
	return nil
}

func (s *Server) endProjectArchive(project string) {
	s.archiveMu.Lock()
	delete(s.projectArchiving, project)
	s.signalArchiveChangedLocked()
	s.archiveMu.Unlock()
}

func (s *Server) endAgentArchive(project, agent string) {
	s.archiveMu.Lock()
	s.endAgentArchiveLocked(project, agent)
	s.archiveMu.Unlock()
}

func (s *Server) endAgentArchiveLocked(project, agent string) {
	delete(s.agentArchiving, agent)
	if s.projectTransitions[project] <= 1 {
		delete(s.projectTransitions, project)
	} else {
		s.projectTransitions[project]--
	}
	s.signalArchiveChangedLocked()
}

func (s *Server) signalArchiveChangedLocked() {
	close(s.archiveChanged)
	s.archiveChanged = make(chan struct{})
}

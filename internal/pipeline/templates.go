package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/agentdeck/agentdeck/internal/config"
)

var (
	ErrTemplateExists   = errors.New("pipeline: template already exists")
	ErrTemplateNotFound = errors.New("pipeline: template not found")
)

// TemplateStore owns validated template CRUD while config.Store owns the file
// paths and durable atomic-write primitive.
type TemplateStore struct {
	config *config.Store
	mu     sync.Mutex
}

func NewTemplateStore(store *config.Store) *TemplateStore {
	return &TemplateStore{config: store}
}

func (s *TemplateStore) List() ([]TemplateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, err := s.config.ListPipelineIDs()
	if err != nil {
		return nil, err
	}
	roles, err := s.roleSet()
	if err != nil {
		return nil, err
	}
	out := make([]TemplateRecord, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.readLocked(id, roles))
	}
	return out, nil
}

func (s *TemplateStore) Read(id string) (TemplateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !config.ValidSlug(id) {
		return TemplateRecord{}, ErrTemplateNotFound
	}
	roles, err := s.roleSet()
	if err != nil {
		return TemplateRecord{}, err
	}
	record := s.readLocked(id, roles)
	if len(record.Diagnostics) == 1 && record.Diagnostics[0].Code == "not_found" {
		return TemplateRecord{}, ErrTemplateNotFound
	}
	return record, nil
}

func (s *TemplateStore) Validate(id string, template Template) TemplateRecord {
	template = NormalizeTemplate(template)
	diagnostics := []Diagnostic{}
	roles, err := s.roleSet()
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{Field: "", Code: "role_read_failed", Message: "configured roles could not be read"})
	} else {
		diagnostics = ValidateTemplate(id, template, roles)
	}
	return TemplateRecord{ID: id, Template: template, Valid: len(diagnostics) == 0, Diagnostics: diagnostics}
}

func (s *TemplateStore) Create(id string, template Template) (TemplateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.config.ReadPipelineFile(id, MaxTemplateBytes); err == nil || !errors.Is(err, config.ErrNotFound) {
		if err == nil || errors.Is(err, config.ErrTooLarge) {
			return TemplateRecord{}, ErrTemplateExists
		}
		return TemplateRecord{}, err
	}
	ids, err := s.config.ListPipelineIDs()
	if err != nil {
		return TemplateRecord{}, err
	}
	if len(ids) >= MaxTemplates {
		return TemplateRecord{}, fmt.Errorf("pipeline: at most %d templates are allowed", MaxTemplates)
	}
	return s.writeLocked(id, template)
}

func (s *TemplateStore) Update(id string, template Template) (TemplateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.config.ReadPipelineFile(id, MaxTemplateBytes); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return TemplateRecord{}, ErrTemplateNotFound
		}
		if !errors.Is(err, config.ErrTooLarge) {
			return TemplateRecord{}, err
		}
	}
	return s.writeLocked(id, template)
}

func (s *TemplateStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.config.ReadPipelineFile(id, MaxTemplateBytes); err != nil && errors.Is(err, config.ErrNotFound) {
		return ErrTemplateNotFound
	}
	return s.config.DeletePipelineFile(id)
}

func (s *TemplateStore) writeLocked(id string, template Template) (TemplateRecord, error) {
	template = NormalizeTemplate(template)
	roles, err := s.roleSet()
	if err != nil {
		return TemplateRecord{}, err
	}
	diagnostics := ValidateTemplate(id, template, roles)
	record := TemplateRecord{ID: id, Template: template, Valid: len(diagnostics) == 0, Diagnostics: diagnostics}
	if !record.Valid {
		return record, nil
	}
	data, err := json.Marshal(template)
	if err != nil {
		return TemplateRecord{}, fmt.Errorf("pipeline: marshal template: %w", err)
	}
	if len(data) > MaxTemplateBytes {
		record.Valid = false
		record.Diagnostics = []Diagnostic{{Field: "", Code: "too_large", Message: fmt.Sprintf("template must be at most %d bytes", MaxTemplateBytes)}}
		return record, nil
	}
	if err := s.config.WritePipelineJSON(id, template); err != nil {
		return TemplateRecord{}, err
	}
	return record, nil
}

func (s *TemplateStore) readLocked(id string, roles map[string]bool) TemplateRecord {
	record := TemplateRecord{ID: id, Template: NormalizeTemplate(Template{}), Diagnostics: []Diagnostic{}}
	data, err := s.config.ReadPipelineFile(id, MaxTemplateBytes)
	if err != nil {
		code := "read_failed"
		message := "template could not be read"
		switch {
		case errors.Is(err, config.ErrNotFound):
			code, message = "not_found", "template does not exist"
		case errors.Is(err, config.ErrTooLarge):
			code, message = "too_large", fmt.Sprintf("template must be at most %d bytes", MaxTemplateBytes)
		}
		record.Diagnostics = []Diagnostic{{Field: "", Code: code, Message: message}}
		return record
	}
	if err := json.Unmarshal(data, &record.Template); err != nil {
		record.Template = NormalizeTemplate(record.Template)
		record.Diagnostics = []Diagnostic{{Field: "", Code: "invalid_json", Message: "template is not valid JSON"}}
		return record
	}
	record.Template = NormalizeTemplate(record.Template)
	record.Diagnostics = ValidateTemplate(id, record.Template, roles)
	record.Valid = len(record.Diagnostics) == 0
	return record
}

func (s *TemplateStore) roleSet() (map[string]bool, error) {
	roles, err := s.config.ListRoles()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(roles))
	for id := range roles {
		out[id] = true
	}
	return out, nil
}

// Digest returns the stable digest used by exact-payload proposal confirmation.
func Digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

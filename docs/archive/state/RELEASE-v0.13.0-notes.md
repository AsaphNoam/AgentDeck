# Chuck v0.13.0

- Organize agents with searchable group selection, drag agents between desktop groups, and
  Stop or Archive a group's agents within one project. Phone groups can collapse and regroup live.
- Refreshed Settings, agent conversations and cards, Think Tank rooms and room cards, and
  lighter buttons across Core, Sky & Grove and Studio.
- Phone conversations now show Working, retain older messages during pagination, and open file
  previews beneath the selected file. Manage controls and the composer are more compact.
- Fixed retained Think Tank room cards collapsing in Archive, and improved context-token counts
  and stopped-agent presentation.

Apple-silicon macOS only. The installer verifies the archive against its SHA-256 manifest.
Artifacts are unsigned and not notarized; macOS may require developer approval.
Provider sign-in is required. Use `chuck update` to update explicitly and `chuck update --rollback`
to restore the previous Chuck runtime. Updating does not restart a running dashboard.

Both Go test variants, UI tests/style checks, static analysis and the distributable build are
release gates. Credentialed Claude/Codex login/chat and the other manual gates in the live handoff
remain owed. Pending independent group, button, room-page and mobile-detail reviews were waived
for publication, not marked complete. Persona and quota-continuation specifications are planned
work and are not shipped in this version.

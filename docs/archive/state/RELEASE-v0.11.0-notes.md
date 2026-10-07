Think Tank workspace controls and quieter chat history make it easier to supervise ongoing agent work.

- Think Tank room cards now appear above project agents with participant rosters, remaining turn allowances and member filtering. Rooms can be titled, and newly deployed participants can share a title group.
- Independent openings run concurrently while publishing in participant order. Shared messages can address participants, turn ceilings can be raised during discussion, and the judge's exact synthesis remains available in its own chat. Failed synthesis reads offer Retry, and retained results remain readable when the source transcript is unavailable.
- Completed chat turns keep the answer prominent and collect thoughts, tools and intermediate activity behind one expandable control across desktop, phone, Archive and Think Tank activity. Pending approvals and important outcomes stay visible. Phone reconnects and sliding history preserve thought ownership and turn identity.
- Text fields grow automatically, Send/Cancel/Collapse use compact icon controls, entity labels are clearer, and archived projects leave the Tasks overview.
- Provider notices appear in the transcript, tools backgrounded by Steer show as continuing work, and refused model changes report the provider's reason. Bundled adapters move to Claude ACP 0.85.1 and Codex ACP 2.1.1+chuck.1, with Codex CLI 0.159.3. The optional source installer also uses Claude ACP 0.85.1.

For Apple-silicon Macs. The archive checksum is published in `manifest.json`; releases are not signed or notarized. macOS may require approval of an unidentified developer on first open.

The repository is still `AsaphNoam/AgentDeck`. Until it is renamed to `AsaphNoam/Chuck`, install with `CHUCK_REPO=AsaphNoam/AgentDeck` or update with `chuck update --repo AsaphNoam/AgentDeck`. Existing AgentDeck installations should follow [the supervised cutover instructions](https://github.com/AsaphNoam/AgentDeck/blob/v0.11.0/docs/chuck-cutover.md); paired phones must re-pair at the Chuck address.

Installed providers remain the default; Bundle is an explicit choice. Installed CLIs update through their own installers, while bundled providers update with Chuck. Updating preserves backend choices and user data; restart the dashboard to use the new runtime. Provider sign-in is still required. Use `chuck update --check` to inspect an update and `chuck update --rollback` for the preceding application release; rollback does not downgrade native provider sessions.

Credentialed Claude/Codex login, compatibility, Think Tank and prompt-adoption checks remain owed, along with the documented phone, notification and cutover manual checks. Automated checks do not stand in for these receipts.

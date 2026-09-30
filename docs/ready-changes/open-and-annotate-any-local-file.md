# Open and annotate any local text file

**State:** Waiting to start
**Why:** Direct operator request after investigating the shipped outside-workspace refusal; the
operator explicitly accepted unrestricted local API reads and durable annotation capture.
**Relevant requirements:** FS-03.R64–R65/A45–A46, FS-13.R25/A16, TS-02.R38, TS-03.R48–R49,
TS-05.R24, TS-08.R80, TS-13.R5/R16, INV §1/§2/§8/§10/§11/§14–§17

## Outcome

A person can open any regular UTF-8 file the AgentDeck process can read from a conversation file
link or local file address, then select its displayed text and assign a point-in-time annotation
through the same tray and target flow used by conversation annotations.

## Included work

Widen the existing loopback file-read route for absolute, relative, symlinked, and `.git` paths
while retaining read-only, regular-file, UTF-8, and size bounds. Keep file reads denied remotely.
Add a backward-compatible tagged file anchor to the existing annotation payload, formatter, card,
store, tray, and delivery path. Source selections include line ranges; rendered Markdown selections
are path-only. Preserve legacy transcript/diff annotations unchanged. Do not add file browsing,
broader composer search, writing, downloads, remote file reads, whole-file annotation, or a second
tray or endpoint.

## How we will know it works

FS-03.A45–A46 and FS-13.A16 cover unrestricted live/archive reads, typed refusals, remote denial,
source and rendered selections, mixed annotation batches, reload/replacement stability, durable
cards and delivery, legacy compatibility, and focused J3/J13 browser journeys. The TS requirements
add independent server path/security, annotation wire/persistence, formatter, UI selection, and
remote-inventory tests.

## Waiting on

None.

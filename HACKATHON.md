# Hackathon record

ProofMuse is Jeejak's entry in the Meta Global AI Developer Hackathon
(Meta x Cerebral Valley, Oct 26 to Nov 4, 2026), Startup track.
This file is the line between what was built during the event and what
existed before. Judges: read this before the README.

## Built during the hackathon (Oct 26 to Nov 4, 2026)

Everything in this repository except the items in the next section.
The git history starts on Oct 26, 2026 and is the evidence.

Planned scope, ticked as it lands:

- [ ] Muse Spark 1.3 structured observation of packing photos (JSON schema, strict)
- [ ] SAM 3.1 segmentation: shipping label crop, sealed-box region, koli count by box instances
- [ ] Second-pass resi read on the SAM label crop, only when the first pass is unsure
- [ ] Go rules engine: schema validation, resi normalisation and order match, verdict, Indonesian messages
- [ ] Retake loop at packing time (bad photo, Indonesian retake prompt, re-verify)
- [ ] Claim triage: buyer's claim text to claim type to the evidence that answers it
- [ ] Dispute packet PDF: verdict, crops and fixed Indonesian strings, no model prose
- [ ] Async jobs with idempotency keys, retries with backoff, per-request usage and cost tracking
- [ ] Eval harness and results table: precision, recall, unsure and invalid rate, false alarms, p50/p95 latency, cost per photo
- [ ] Mobile-friendly web UI with box and mask overlay
- [ ] Stretch: SAM 3.1 video pass over a packing clip
- [ ] Stretch: open-weight comparison column (Muse Spark Open)

## Pre-existing (not judged)

ProofMuse succeeds ParcelProof (https://github.com/jeejak/parcel-proof,
Apache-2.0, built Oct 13 to 18, 2026 for the AI Builder Cup, Gemini-based).
ParcelProof checked photos after they synced to the cloud; ProofMuse
checks them at the packing bench, grounds every finding with SAM 3.1,
reads the buyer's claim and assembles the dispute packet.

Ported from ParcelProof, each adapted for the Meta pipeline and listed
here when it lands:

- (none yet)

Candidates we expect to port, named now so the list cannot grow quietly:
the verdict rules and reason codes (`evidencecheck/verdict.go`), the
no-model basic check for brightness, sharpness and barcode
(`evidencecheck/basic.go`), the eval harness layout (`cmd/eval`), and
the synthetic eval set with its dummy-waybill generator.

Also pre-existing and not part of this submission: the JejakPaket mobile
app and the Jeejak backend (private). ProofMuse does not depend on them.

## Data

Only staged parcel photos with dummy shipping labels. No real customer
photos, names, phone numbers or addresses anywhere in this repository,
the demo or the eval set.

## Team

- Samsul Hadi (representative), Jeejak, CV. Adisatya IT Consultant, Kediri, Indonesia

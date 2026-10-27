# Hackathon record

ProofMuse is Jeejak's entry in the Meta Global AI Developer Hackathon
(Meta x Cerebral Valley, Oct 26 to Nov 4, 2026), Startup track.
This file is the line between what was built during the event and what
existed before. Judges: read this before the README.

## Built during the hackathon (Oct 26 to Nov 4, 2026)

Everything in this repository except the items in the next section.
The git history starts on Oct 26, 2026 and is the evidence.

Planned scope, ticked as it lands:

- [x] Muse Spark 1.3 structured observation of packing photos, per photo (JSON schema, strict)
- [x] SAM 3.1 label grounding on every photo that may show a label (box for overlay and crop)
- [ ] SAM 3.1 koli count by carton instances, sealed-box region
- [x] Second-pass resi read on the SAM label crop, only when the first pass could not read it
- [x] Verify before accusing: a resi that differs from the order is read again from the crop; the mismatch stands only if both reads agree
- [x] Go rules engine: schema validation, resi normalisation and order match, verdict, Indonesian messages
- [x] Retake loop at packing time (bad photo, Indonesian retake prompt, retake one photo, check again, attempt history)
- [x] Claim triage: buyer's message to claim type (Muse Spark, enum only) to the photos that answer it and what they cannot show (Go rules)
- [ ] Dispute packet PDF: verdict, crops and fixed Indonesian strings, no model prose
- [x] Server-to-server API (`POST /v1/observe`, keyed, per-caller limits) and a dependency-free Go client (`client/`) returning the six-field observation the Jeejak console stores
- [x] Retries with backoff and Retry-After, per-call usage and cost tracking
- [x] Async jobs with idempotency keys (order + photo hashes + prompt + config), results persisted, per-client and daily rate limits
- [x] Eval harness and results table: precision, recall, invalid rate, false alarms, per-photo field agreement, label-box IoU, p50/p95 latency, cost per parcel and per photo
- [x] Mobile-friendly web UI with the SAM label box overlay and the step trace
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

- `evalset/records/R01` to `R64`: the 279 synthetic photos generated on
  2026-10-07 for ParcelProof, copied and resized to 1280 px JPEG, plus the
  original 11 label columns in `evalset/labels.csv`. Everything else in
  `evalset/` (R65 to R104, photo-level labels, orders, claims, retake
  pairs, split, review page, tools) was built during the hackathon. See
  `evalset/README.md`.
- `internal/evidence/decide.go`: `Decide` and `NormaliseResi` ported from
  ParcelProof `evidencecheck/verdict.go` (the verdict rule and the nine
  reason codes). New in ProofMuse: per-photo observations folded by
  `Aggregate`, and the `item_mismatch` and `resi_conflict` reasons.
- `internal/evidence/messages.go`: the nine Indonesian problem strings
  from ParcelProof's demo page. The retake instructions, English lines
  and the two new reasons' strings are new.

Candidates we may still port, named now so the list cannot grow quietly:
the no-model basic check for brightness, sharpness and barcode
(`evidencecheck/basic.go`). The eval harness in `cmd/eval` was written
fresh for the per-photo pipeline; it keeps ParcelProof's idea of scoring
bukti lemah precision and recall against human labels.

Also pre-existing and not part of this submission: the JejakPaket mobile
app and the Jeejak backend (private). ProofMuse does not depend on them.

## Data

Only staged parcel photos with dummy shipping labels. No real customer
photos, names, phone numbers or addresses anywhere in this repository,
the demo or the eval set.

## Team

- Samsul Hadi (representative), Jeejak, CV. Adisatya IT Consultant, Kediri, Indonesia

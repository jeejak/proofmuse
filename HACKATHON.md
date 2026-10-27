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
(`evidencecheck/basic.go`), and the eval harness layout (`cmd/eval`).

Also pre-existing and not part of this submission: the JejakPaket mobile
app and the Jeejak backend (private). ProofMuse does not depend on them.

## Data

Only staged parcel photos with dummy shipping labels. No real customer
photos, names, phone numbers or addresses anywhere in this repository,
the demo or the eval set.

## Team

- Samsul Hadi (representative), Jeejak, CV. Adisatya IT Consultant, Kediri, Indonesia

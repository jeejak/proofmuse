# ProofMuse synthetic packing-evidence evaluation fixtures

All images are synthetic, generated with an image model. They are not camera photos and the numbers measured on them are not accuracy on real packing photos.

R01 to R64 were generated on 2026-10-07 for ParcelProof and are pre-existing. Everything from R65 on, and every file except the copied photos and the original label columns, was built during the hackathon. New assets were generated on 2026-10-26. Source photos were copied and resized to JPEG with a 1280 pixel long side; the source directories were not edited.

**Human review, 2026-10-27.** Samsul reviewed all 399 photos in the review page, 397 in blind mode (his values entered before the proposals were shown) and 2 in assisted mode. He changed 42 photos' fields, mostly whether a sealed box is shown, and redrew 200 label boxes. His decisions are kept verbatim in `review/review-decisions.csv`. Every photo row now has `needs_review=no`.

**Record labels are derived, not proposed.** No record row was reviewed directly. `tools/derive_records.py` recomputes every record column (`image_usable`, `label_visible`, `label_resi_read`, `package_sealed`, `item_visible`, `koli_seen`, `human_verdict`, `verdict_reasons`) from the reviewed photo labels with fixed rules written at the top of that script, and marks the row `record_fields_source=derived_from_human_photos`. The verdict applies ParcelProof's rules to those values. One exception rests on construction rather than review: the four `item_mismatch` records carry an `item_mismatch` reason because the photo shows a different item from the order listing, which no photo field records. Derivation moved 8 verdicts from the earlier intended values; for example R47, R69 and R70 lose their resi read because no photo shows a readable label, and five staged obstructions become usable because the obstruction covers only some photos. `tools/verify.py` fails if a derived row is out of date.

The waybills are dummies; no real person's shipping data appears anywhere. New labels were visually checked for unwanted names, telephone numbers and addresses. Four first shots (R79/01, R81/01, R83/01 and R85/01) were regenerated as a precaution because unrequested paper slips contained unverifiable tiny text. R87/02 was regenerated separately to correct its courier logo. See `tools/regeneration-log.jsonl`. The inherited images include printed book-cover author text and, in some scenes, hands. Those are pre-existing synthetic content, not customer records. No real customer data was supplied. This is a documented scope limitation to any broader claim that no person-like text appears anywhere in the images.

## Contents and provenance

- `records/`: 104 records, 399 unique synthetic JPEGs. The original 279 photos plus 120 new photos, three per new record.
- `labels.csv`: original 11 columns preserved for R01 to R64; group, retake link, mock listing, and review provenance added. New record fields are pixel-informed proposals.
- `photos.csv`: one proposal per photo. The `synthetic` column marks fixtures explicitly. Boxes count shipping cartons, not retail product packaging. A sealed box means at least one visibly taped-closed carton. A label box encloses the main visible waybill on the 0 to 1000 coordinate grid; it is approximate, not a SAM segmentation.
- `orders.json`: 104 mock orders, 62 Shopee and 42 TikTok. Order resi values preserve the original intended numbers even for wrong-label cases. Listing text for inherited records comes from the original shot list and is not an independently verified product catalog.
- `claims.csv`: 40 synthetic informal Indonesian buyer claims, eight per type. Photo references provide relevant packing context, not proof that a buyer received an intact parcel. Explicit gaps cover missing delivery, continuity and quantity evidence.
- `retake_pairs.csv`: eight bad/good pairs sharing the same intended item, courier and resi. Reference images were used to preserve continuity, but generated texture, angle and box details can drift. These are staged synthetic comparisons, not physical repeat captures.
- `split.csv`: frozen 42-record dev and 62-record test split. Each verdict and group appears in both, and retake pairs stay together. Seed: `20261026`. Marketplace seed: `20261027`. Prompt tuning may only inspect dev. The separate human review queue intentionally starts with test as requested; do not use that review session to tune prompts.
- `PROVENANCE.jsonl`: actual generation prompt, source image path, input references where recorded, original and output hashes, and `synthetic=true` for every new photo. Source PNG paths are local provenance pointers, not required at evaluation time.
- `review/notes.md`: all uncertain fields, visual limitations and prompt-to-image differences. Prompts describe intent; the photo proposals describe visible results. For example, some dark or distant views still have readable resi values.
- `tools/`: generation plan, immutable source hash snapshot, inspected proposals, deterministic fixture builders, review merger, checks and tests. No labeling or grading API is called by these scripts.

## Use and review

Use Python with `Pillow==12.3.0`, pinned in `tools/requirements.txt`. From this directory:

```sh
python3 tools/build_metadata.py
python3 tools/verify.py
python3 tools/test_review.py
node tools/test_review_ui.cjs
python3 tools/test_rebuild.py
```

Open `review/index.html` directly in a browser. It has no server, external scripts or network calls. Blind mode starts on and hides proposal values and boxes. Enter your own labels and bounding box before saving; disagreements then become visible. Use the full-size image link for tiny labels. The queue starts with test records, followed by dev. Browser local storage is only a convenience; download `review-decisions.csv` to preserve decisions.

After Samsul's review:

```sh
python3 tools/apply_review.py /absolute/path/to/review-decisions.csv
python3 tools/verify.py
```

Only explicitly reviewed photo rows change to `needs_review=no`, with the submitted reviewer and ISO timestamp in WIB (`+07:00`). `label_source=human` means every photo of the record was reviewed. It does not silently validate the original record verdict: the optional independent record review also sets `record_fields_source=human`. Report record-verdict agreement only for records with both fields set to human; report photo agreement only on reviewed photo rows. No accuracy, false-alarm, latency or cost result is claimed here. Review merging and blind-mode/export logic passed offline tests. Metadata rebuilds are byte-stable and frozen-image replay was tested. The UI logic test uses a fake DOM; visual browser rendering was not tested because the in-app browser blocks local file URLs.

`build_metadata.py` preserves completed photo reviews and an existing frozen split. It rebuilds derived files deterministically from saved inputs. `prepare.py` is an initial import tool and refuses to reset an existing set. `ingest_image.py` deterministically resizes an already generated source asset and records its hash; fresh image-model generation itself is not deterministic. The committed-to-disk prompts and source assets are the replay boundary. Nothing has been committed or pushed by this preparation task.

The privacy checker scans all file bytes for email and standalone Indonesian phone-like tokens. Token boundaries avoid treating a substring inside a courier tracking number as a phone. Byte scanning does not read image text; visual waybill inspection complements it. Four JPEGs initially contained accidental email-shaped sequences in compressed entropy bytes (R32/02, R48/01, R86/03 and R101/03). Deterministic equivalent entropy encoding removed those sequences; decoded pixels were asserted identical. The encoding policy is shared by import, ingest and replay tools. Original source hashes are checked against `tools/source-snapshot.json` on every verification.

## Verification counts

The exact output of `tools/verify.py` is recorded below.

<!-- SYNTHETIC-COUNTS -->
```json
{
  "synthetic": true,
  "records": 104,
  "photos": 399,
  "pre_existing_photos": 279,
  "new_photos": 120,
  "verdict": {
    "kuat": 42,
    "lemah": 36,
    "perlu_dicek": 26
  },
  "group": {
    "blurry": 6,
    "dark": 6,
    "glare": 4,
    "good": 13,
    "icp_items": 6,
    "item_mismatch": 4,
    "item_never_shown": 5,
    "koli_mismatch": 4,
    "label_unreadable": 4,
    "label_with_a_different_resi": 6,
    "mixed": 6,
    "never_sealed": 4,
    "no_label": 5,
    "obstructed": 5,
    "portrait": 6,
    "retake_pairs": 16,
    "too_far_cropped": 4
  },
  "split": {
    "dev": 42,
    "test": 62
  },
  "claim_type": {
    "barang_kurang": 8,
    "barang_rusak": 8,
    "paket_kosong": 8,
    "salah_kirim": 8,
    "tidak_diterima": 8
  },
  "claims_on_weak_records": 23,
  "retake_pairs": 8,
  "needs_review": 0
}
```

PASS: synthetic fixture checks. This is not a real-photo accuracy result.

# Eval: spark-open on test

Synthetic photos with dummy waybills, scored against Samsul's human photo labels (record labels derived from them). Not accuracy on real packing photos.

Config: vision `meta-models/muse-spark-open-1026` effort `minimal`, photos at 1024 px, SAM `sam-3.1` ground=true reread=true verify=true, prompt `2026-10-27.1`.

| Measure | Result |
|---|---|
| Records scored | 62 of 62 (errors 0, invalid after retry 0) |
| Verdict agrees with human | 83.9% (52/62) |
| Bukti lemah precision | 88.2% (15/17) |
| Bukti lemah recall | 71.4% (15/21) |
| False alarms on human kuat | 8.0% (2/25) |
| Invalid answers (schema, after one retry) | 0.0% (0/62) |
| Latency per parcel p50 / p95 | 10.9 s / 16.6 s |
| Cost per parcel mean / p95 | $0.0138 / $0.0186 |
| Cost per photo | $0.0036 |
| Tokens per parcel in / out (reasoning) | 4731 / 1089 (619) |

Confusion (rows human, columns model):

| human \ model | kuat | perlu_dicek | lemah |
|---|---|---|---|
| kuat | 23 | 0 | 2 |
| perlu_dicek | 2 | 14 | 0 |
| lemah | 3 | 3 | 15 |

Per-photo agreement, first pass (photos the human marked uncertain are left out and counted):

| Field | Agreement | Human uncertain |
|---|---|---|
| label_in_photo | 100.0% (236/236) | 0 |
| label_readable | 97.5% (230/236) | 0 |
| sealed_box_in_photo | 90.2% (211/234) | 2 |
| item_in_photo | 30.9% (73/236) | 0 |
| boxes_in_photo | 100.0% (236/236) | 0 |
| image_quality_issues | 91.1% (215/236) | 0 |

Label boxes against the human box (IoU on the 0 to 1000 grid, photos with a human-labelled label):

| Source | Mean IoU | IoU >= 0.5 |
|---|---|---|
| Muse Spark first pass | 0.87 | 97.5% (115/118) |
| SAM 3.1 (`label`) | 0.90 | 95.8% (113/118) |

SAM missed a human-labelled label on 0 grounded photos and found one where the human saw none on 0.
Pipeline steps: map[ground:236 observe:62 reread:30 verify:9]. Rereads that read an unread label: 1. Verify: 1 mismatches confirmed, 8 withdrawn.

Disagreements:

- R29 (obstructed): human kuat [], model lemah [image_obstructed]
- R36 (label_with_a_different_resi): human lemah [label_mismatch], model perlu_dicek [label_unreadable]
- R38 (label_with_a_different_resi): human lemah [label_mismatch], model perlu_dicek [label_unreadable]
- R40 (label_with_a_different_resi): human lemah [label_mismatch], model perlu_dicek [label_unreadable]
- R43 (item_never_shown): human lemah [item_not_visible], model kuat []
- R44 (item_never_shown): human lemah [item_not_visible], model kuat []
- R45 (item_never_shown): human lemah [item_not_visible], model kuat []
- R47 (label_unreadable): human perlu_dicek [label_unreadable], model kuat []
- R69 (too_far_cropped): human perlu_dicek [label_unreadable], model kuat []
- R71 (too_far_cropped): human kuat [], model lemah [item_not_visible]

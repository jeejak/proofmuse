# Eval: spark-only on test

Synthetic photos with dummy waybills, scored against Samsul's human photo labels (record labels derived from them). Not accuracy on real packing photos.

Config: vision `muse-spark-1.3` effort `minimal`, photos at 1024 px, SAM `sam-3.1` ground=false reread=false verify=false, prompt `2026-10-27.1`.

| Measure | Result |
|---|---|
| Records scored | 62 of 62 (errors 0, invalid after retry 0) |
| Verdict agrees with human | 95.2% (59/62) |
| Bukti lemah precision | 95.5% (21/22) |
| Bukti lemah recall | 100.0% (21/21) |
| False alarms on human kuat | 0.0% (0/25) |
| Invalid answers (schema, after one retry) | 0.0% (0/62) |
| Latency per parcel p50 / p95 | 2.8 s / 5.7 s |
| Cost per parcel mean / p95 | $0.0065 / $0.0117 |
| Cost per photo | $0.0017 |
| Tokens per parcel in / out (reasoning) | 3938 / 698 (321) |

Confusion (rows human, columns model):

| human \ model | kuat | perlu_dicek | lemah |
|---|---|---|---|
| kuat | 25 | 0 | 0 |
| perlu_dicek | 2 | 13 | 1 |
| lemah | 0 | 0 | 21 |

Per-photo agreement, first pass (photos the human marked uncertain are left out and counted):

| Field | Agreement | Human uncertain |
|---|---|---|
| label_in_photo | 99.6% (235/236) | 0 |
| label_readable | 97.5% (230/236) | 0 |
| sealed_box_in_photo | 99.1% (232/234) | 2 |
| item_in_photo | 98.7% (233/236) | 0 |
| boxes_in_photo | 100.0% (236/236) | 0 |
| image_quality_issues | 91.9% (217/236) | 0 |

Label boxes against the human box (IoU on the 0 to 1000 grid, photos with a human-labelled label):

| Source | Mean IoU | IoU >= 0.5 |
|---|---|---|
| Muse Spark first pass | 0.79 | 94.9% (112/118) |

Disagreements:

- R47 (label_unreadable): human perlu_dicek [label_unreadable], model kuat []
- R49 (label_unreadable): human perlu_dicek [label_unreadable], model lemah [label_mismatch]
- R69 (too_far_cropped): human perlu_dicek [label_unreadable], model kuat []

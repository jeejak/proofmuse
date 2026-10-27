# Eval: spark-only on dev

Synthetic photos with dummy waybills, scored against Samsul's human photo labels (record labels derived from them). Not accuracy on real packing photos.

Config: vision `muse-spark-1.3` effort `minimal`, photos at 1024 px, SAM `sam-3.1` ground=false reread=false verify=false, prompt `2026-10-27.1`.

| Measure | Result |
|---|---|
| Records scored | 42 of 42 (errors 0, invalid after retry 0) |
| Verdict agrees with human | 97.6% (41/42) |
| Bukti lemah precision | 93.8% (15/16) |
| Bukti lemah recall | 100.0% (15/15) |
| False alarms on human kuat | 0.0% (0/17) |
| Invalid answers (schema, after one retry) | 0.0% (0/42) |
| Latency per parcel p50 / p95 | 3.3 s / 5.3 s |
| Cost per parcel mean / p95 | $0.0068 / $0.0114 |
| Cost per photo | $0.0018 |
| Tokens per parcel in / out (reasoning) | 4006 / 737 (353) |

Confusion (rows human, columns model):

| human \ model | kuat | perlu_dicek | lemah |
|---|---|---|---|
| kuat | 17 | 0 | 0 |
| perlu_dicek | 0 | 9 | 1 |
| lemah | 0 | 0 | 15 |

Per-photo agreement, first pass (photos the human marked uncertain are left out and counted):

| Field | Agreement | Human uncertain |
|---|---|---|
| label_in_photo | 99.4% (162/163) | 0 |
| label_readable | 97.5% (159/163) | 0 |
| sealed_box_in_photo | 97.5% (157/161) | 2 |
| item_in_photo | 98.1% (158/161) | 2 |
| boxes_in_photo | 100.0% (163/163) | 0 |
| image_quality_issues | 89.0% (145/163) | 0 |

Label boxes against the human box (IoU on the 0 to 1000 grid, photos with a human-labelled label):

| Source | Mean IoU | IoU >= 0.5 |
|---|---|---|
| Muse Spark first pass | 0.78 | 93.9% (77/82) |

Disagreements:

- R70 (too_far_cropped): human perlu_dicek [label_unreadable], model lemah [label_mismatch]

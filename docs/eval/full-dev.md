# Eval: full on dev

Synthetic photos with dummy waybills, scored against Samsul's human photo labels (record labels derived from them). Not accuracy on real packing photos.

Config: vision `muse-spark-1.3` effort `minimal`, photos at 1024 px, SAM `sam-3.1` ground=true reread=true verify=true, prompt `2026-10-27.1`.

| Measure | Result |
|---|---|
| Records scored | 42 of 42 (errors 0, invalid after retry 0) |
| Verdict agrees with human | 100.0% (42/42) |
| Bukti lemah precision | 100.0% (15/15) |
| Bukti lemah recall | 100.0% (15/15) |
| False alarms on human kuat | 0.0% (0/17) |
| Invalid answers (schema, after one retry) | 0.0% (0/42) |
| Latency per parcel p50 / p95 | 5.3 s / 8.5 s |
| Cost per parcel mean / p95 | $0.0142 / $0.0201 |
| Cost per photo | $0.0037 |
| Tokens per parcel in / out (reasoning) | 4635 / 954 (556) |

Confusion (rows human, columns model):

| human \ model | kuat | perlu_dicek | lemah |
|---|---|---|---|
| kuat | 17 | 0 | 0 |
| perlu_dicek | 0 | 10 | 0 |
| lemah | 0 | 0 | 15 |

Per-photo agreement, first pass (photos the human marked uncertain are left out and counted):

| Field | Agreement | Human uncertain |
|---|---|---|
| label_in_photo | 100.0% (163/163) | 0 |
| label_readable | 97.5% (159/163) | 0 |
| sealed_box_in_photo | 97.5% (157/161) | 2 |
| item_in_photo | 97.5% (157/161) | 2 |
| boxes_in_photo | 100.0% (163/163) | 0 |
| image_quality_issues | 89.0% (145/163) | 0 |

Label boxes against the human box (IoU on the 0 to 1000 grid, photos with a human-labelled label):

| Source | Mean IoU | IoU >= 0.5 |
|---|---|---|
| Muse Spark first pass | 0.78 | 93.9% (77/82) |
| SAM 3.1 (`label`) | 0.88 | 93.8% (76/81) |

SAM missed a human-labelled label on 1 grounded photos and found one where the human saw none on 0.
Pipeline steps: map[ground:163 observe:42 reread:20 verify:9]. Rereads that read an unread label: 0. Verify: 7 mismatches confirmed, 2 withdrawn.

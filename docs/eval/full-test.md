# Eval: full on test

Synthetic photos with dummy waybills, scored against Samsul's human photo labels (record labels derived from them). Not accuracy on real packing photos.

Config: vision `muse-spark-1.3` effort `minimal`, photos at 1024 px, SAM `sam-3.1` ground=true reread=true verify=true, prompt `2026-10-27.1`.

| Measure | Result |
|---|---|
| Records scored | 62 of 62 (errors 0, invalid after retry 0) |
| Verdict agrees with human | 96.8% (60/62) |
| Bukti lemah precision | 100.0% (21/21) |
| Bukti lemah recall | 100.0% (21/21) |
| False alarms on human kuat | 0.0% (0/25) |
| Invalid answers (schema, after one retry) | 0.0% (0/62) |
| Latency per parcel p50 / p95 | 4.6 s / 7.8 s |
| Cost per parcel mean / p95 | $0.0138 / $0.0209 |
| Cost per photo | $0.0036 |
| Tokens per parcel in / out (reasoning) | 4465 / 874 (485) |

Confusion (rows human, columns model):

| human \ model | kuat | perlu_dicek | lemah |
|---|---|---|---|
| kuat | 25 | 0 | 0 |
| perlu_dicek | 2 | 14 | 0 |
| lemah | 0 | 0 | 21 |

Per-photo agreement, first pass (photos the human marked uncertain are left out and counted):

| Field | Agreement | Human uncertain |
|---|---|---|
| label_in_photo | 100.0% (236/236) | 0 |
| label_readable | 97.5% (230/236) | 0 |
| sealed_box_in_photo | 98.3% (230/234) | 2 |
| item_in_photo | 98.3% (232/236) | 0 |
| boxes_in_photo | 100.0% (236/236) | 0 |
| image_quality_issues | 91.1% (215/236) | 0 |

Label boxes against the human box (IoU on the 0 to 1000 grid, photos with a human-labelled label):

| Source | Mean IoU | IoU >= 0.5 |
|---|---|---|
| Muse Spark first pass | 0.78 | 94.1% (111/118) |
| SAM 3.1 (`label`) | 0.90 | 95.8% (113/118) |

SAM missed a human-labelled label on 0 grounded photos and found one where the human saw none on 0.
Pipeline steps: map[ground:236 observe:62 reread:27 verify:10]. Rereads that read an unread label: 3. Verify: 8 mismatches confirmed, 2 withdrawn.

Disagreements:

- R47 (label_unreadable): human perlu_dicek [label_unreadable], model kuat []
- R69 (too_far_cropped): human perlu_dicek [label_unreadable], model kuat []

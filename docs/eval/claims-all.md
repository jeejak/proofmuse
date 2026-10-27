# Eval: buyer claim triage on all

40 synthetic informal Indonesian buyer messages, 8 per type. Expected types are the types each message was written for, not a human review. Model `muse-spark-1.3`, effort `minimal`.

| Measure | Result |
|---|---|
| Claim type agrees | 100.0% (40/40) |
| Errors | 0 of 40 |
| Latency p50 / p95 | 0.8 s / 2.0 s |
| Cost per claim | $0.00064 |

| expected \ model | barang_kurang | barang_rusak | paket_kosong | salah_kirim | tidak_diterima | lainnya |
|---|---|---|---|---|---|---|
| barang_kurang | 8 | 0 | 0 | 0 | 0 | 0 |
| barang_rusak | 0 | 8 | 0 | 0 | 0 | 0 |
| paket_kosong | 0 | 0 | 8 | 0 | 0 | 0 |
| salah_kirim | 0 | 0 | 0 | 8 | 0 | 0 |
| tidak_diterima | 0 | 0 | 0 | 0 | 8 | 0 |

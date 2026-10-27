# ProofMuse by Jeejak

**Will this packing photo hold up in a dispute? Find out while the parcel is still on the table.**

ProofMuse is the successor to [ParcelProof](https://github.com/jeejak/parcel-proof), built on Meta Muse Spark 1.3 and SAM 3.1. An Indonesian online seller photographs the packing, and within seconds gets practical feedback in Indonesian: *Foto label kurang jelas, silakan foto ulang.* The models observe; a Go rules engine decides. The result is advisory. It flags evidence-quality issues for a person to act on and never claims to prove what was in the box or to guarantee a dispute outcome.

Built for the Meta Global AI Developer Hackathon (Meta x Cerebral Valley, 2026), Startup track. See [HACKATHON.md](HACKATHON.md) for what was built during the event and what was ported.

## How it works

1. **Observe.** One Muse Spark 1.3 call labels every photo with a strict JSON schema: is a label visible and readable, the resi as printed, is a sealed carton shown, is the item shown, how many cartons, what quality problems, and where the label is on a 0 to 1000 grid. The model is never told the expected resi.
2. **Ground.** SAM 3.1 segments the label on every photo that may show one. Its box drives the overlay and the crop.
3. **Reread.** Where a label is located but its number was not read, Muse Spark reads it again from a high-resolution SAM crop of the original photo.
4. **Verify before accusing.** Where a read differs from the order, that label is read again from its crop. A mismatch stands only if both reads agree; otherwise the label counts as unreadable ("please check", not "wrong label").
5. **Decide.** Go rules, no model: fold the photos into one record, compare the resi with the order, and return Bukti kuat, Perlu dicek or Bukti lemah with fixed Indonesian reasons and retake instructions.

Every step is written to a trace with why it ran and what it cost.

**Buyer claim triage.** When a complaint comes in, the seller pastes the buyer's message under a finished check. Muse Spark only classifies it into one of five claim types (barang kurang, rusak, paket kosong, salah kirim, tidak diterima) or "other". Go rules then pick the photos that answer that claim, skipping blurry, dark or obstructed ones, and name what packing photos cannot show: item count, condition on arrival, handover to the courier. On the 40 synthetic buyer messages in the eval set, the type matched 40 of 40 at 0.8 s and $0.0006 per claim ([`docs/eval/claims-all.md`](docs/eval/claims-all.md)). The messages were written to be clear examples of each type, so this shows the pipeline works, not robustness to messy real chats.

## Measured, not claimed

Test split, 62 records, run once per configuration (`go run ./cmd/eval -split test`). Reports: [`docs/eval/`](docs/eval/).

| | Full pipeline | Muse Spark only |
|---|---|---|
| Verdict agrees with human | 96.8% (60/62) | 95.2% (59/62) |
| Bukti lemah precision | **100.0%** (21/21) | 95.5% (21/22) |
| Bukti lemah recall | 100.0% (21/21) | 100.0% (21/21) |
| False alarms on human "kuat" | 0.0% (0/25) | 0.0% (0/25) |
| Invalid answers | 0.0% (0/62) | 0.0% (0/62) |
| Latency per parcel p50 / p95 | 4.6 s / 7.8 s | 2.8 s / 5.7 s |
| Cost per parcel | $0.0138 | $0.0065 |

What the extra steps buy: without them, Muse Spark misread one digit on a small label and the rules called it the wrong label, the most alarming thing a seller can be told. Verify-before-accusing withdrew that read on the test split (R49) and on the dev split (R70), and confirmed every genuine wrong label.

**Honest limits.**

- **The photos are synthetic.** All 399 are AI-generated images of staged packing with dummy waybills, not camera photos, so these numbers are not accuracy on real packing photos.
- **The labels come from one person.** Samsul labelled every photo blind, before seeing any model output. Record labels are derived from those photo labels by fixed rules ([`evalset/README.md`](evalset/README.md)).
- **The two disagreements are about legibility.** On R47 and R69 the human marked the label unreadable, while the model read it and matched the order's resi exactly. The labels were left as they are.
- **The dev split is not pristine.** It was looked at during development, so the test split above is the published number.

## About Jeejak

Jeejak is a family of small-business tools for Indonesian online sellers, made by CV. Adisatya IT Consultant in Kediri, East Java ([jeejak.id](https://jeejak.id)). Its first product, [JejakPaket](https://play.google.com/store/apps/details?id=id.jeejak.paket), records proof for every parcel a Shopee or TikTok Shop seller sends: the resi scan, packing photos, the handover video and a dispute PDF. Paid plans sync that evidence to a seller console that matches it with marketplace orders.

JejakPaket records evidence; ProofMuse checks it. It is the AI layer we built during this hackathon, and it is meant to run inside JejakPaket at packing time once it has been measured on real photos from sellers who opt in. JejakPaket and the console are pre-existing and are not part of this submission; see [HACKATHON.md](HACKATHON.md).

## Run locally

```bash
cp .env.example .env    # add MODEL_API_KEY
go run ./cmd/server     # http://localhost:8080/health
```

## License

Apache-2.0. Copyright 2026 CV. Adisatya IT Consultant. Jeejak, JejakPaket and ParcelProof are its trademarks.

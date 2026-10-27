# Demo video script (2:55, hard maximum 3:00)

The submission asks for a **three-minute** demo video that highlights **only the features, code and functionality built during the hackathon**; judges must be able to tell what is new, or the entry can be disqualified. So:

- **Aim for 2:50 to 2:57, never above 3:00.** Never below 2:40.
- **The live app fills at least 1:40 of the running time.** Show it in a browser with the address bar readable. Motion graphics explain; the app proves.
- **Jeejak and JejakPaket appear once, for under 15 seconds, captioned "Existing product, not part of this submission".** No JejakPaket app or seller-console footage: they are pre-existing.
- **Speak, and caption the same words.** About 2.3 words a second, unhurried. Judges often watch muted.

Live URL: the deployed demo (see `docs/deploy.md`). Local fallback: `go run ./cmd/server`, then http://localhost:8080.

| Time | On screen | Say |
|---|---|---|
| 0:00 to 0:16 | A dark packing photo, then a blurry one, then the text "Dispute lost. The parcel is gone." | "Indonesian online sellers lose Shopee and TikTok Shop disputes because their packing photo turns out useless: too dark, label cut off, box never shown sealed. They find out days later, when nothing can be re-shot." |
| 0:16 to 0:30 | Title card: Jeejak logo, "JejakPaket, our packing-evidence app (existing product, not part of this submission)", then the ProofMuse title with "Built this week on Meta Muse Spark and SAM 3.1" | "We're Jeejak, from Kediri. Our app JejakPaket already records packing evidence. This week we built ProofMuse on Meta's models: it checks that evidence while the parcel is still on the table." |
| 0:30 to 1:00 | **Browser, address bar visible.** Sample **Foto gelap**. Press **Cek bukti**. Result: Bukti lemah, "Foto terlalu gelap" with the retake instruction; the green SAM label boxes on the photos | "A seller packs a mug and takes three photos. ProofMuse answers in about five seconds: Bukti lemah, weak evidence. The photos are too dark. It says why, and what to do, in Indonesian. The green boxes are SAM 3.1 finding the shipping label on each photo." |
| 1:00 to 1:20 | Press **Contoh: pakai foto ulang**, then **Cek lagi**. Bukti kuat; the resi under each photo; **Riwayat**: #1 Bukti lemah, #2 Bukti kuat | "The seller retakes in better light and checks again. Bukti kuat. The tracking number is read from the label and matches the order. That is the whole point: the fix takes seconds at the bench, not an argument days later." |
| 1:20 to 1:45 | Sample **Label milik pesanan lain**, **Cek bukti**, Bukti lemah "Nomor resi di label beda dengan pesanan". Open **Di balik layar**, highlight the two **verify … confirmed: both reads agree** rows | "Here the wrong label went on the box. The model is never told the expected number; it only reads. Before ProofMuse may say wrong label, it reads that label again from a high-resolution SAM crop. Both reads agree, so the mismatch stands." |
| 1:45 to 2:08 | Back on the first sample's retaken set. Section 4, tap **kak paketnya kosong**, **Baca komplain**. "Paket kosong", photos highlighted as answering, the gap "video packing lebih kuat" | "When a buyer complains, the seller pastes the message. Muse Spark only classifies it: empty box. Our rules pick the photos that answer it, and say honestly what photos cannot show." |
| 2:08 to 2:36 | The README results table, then the line "Without verify: one misread became a false wrong-label" | "We measured it on a held-out test set of 62 parcels, labelled blind by a person. Every weak verdict was right and every weak parcel was caught, with no false alarms on strong evidence, at under one and a half US cents a parcel. Without the verify step, a one-digit misread became a false wrong-label. With it, none did." |
| 2:36 to 2:50 | Architecture: photos, Muse Spark observe, SAM 3.1 ground, reread, verify, Go rules, verdict; small tags "async, idempotent, rate-limited, cost per call" | "Muse Spark observes, SAM grounds, and plain Go rules decide. It runs as idempotent background checks with rate limits and cost tracking. It is advice, not proof: the marketplace still decides." |
| 2:50 to 2:57 | End card: ProofMuse by Jeejak, github.com/jeejak/proofmuse, the live URL, jeejak.id | "ProofMuse, by Jeejak. Next: the check inside JejakPaket, with sellers in a pilot." |

The voice is 328 words, about 2:23 of speech at 2.3 words a second; every slot is at least as long as its line. The remaining half minute is clicks, results held on screen and the end card. If the edit runs over 2:58, trim silent waits first, never the measured numbers or the verify rows. Label any sped-up wait "sped up".

## Words to keep exact

Each is checked against `docs/eval/full-test.md` and `docs/eval/spark-only-test.md`. Do not round or rephrase them on screen or in the voice.

- **"62 parcels, held-out test set, labelled blind by a person."** The photos are AI-generated synthetic images with dummy waybills: say "synthetic" on screen in the results shot.
- **"Every weak verdict was right and every weak parcel was caught"** = 100% precision (21/21) and 100% recall (21/21) on Bukti lemah. **"No false alarms on strong evidence"** = 0 of 25.
- **"Under one and a half US cents a parcel"** = $0.0138 (1.4 cents). Latency is 4.6 s median; say "about five seconds".
- **"Without the verify step, a one-digit misread became a false wrong-label"**: Muse Spark alone flagged R49 as label_mismatch on the test split (95.5% lemah precision). Say "one", not "several".
- **Never "win the dispute".** It checks whether the evidence would hold up; the marketplace decides.
- **Model names:** Muse Spark 1.3 and SAM 3.1, through the Meta Model API.

## Before recording

1. Open the live URL in a fresh window; `/health` answers `ok`.
2. Run every sample used above once before recording, so each answer comes from the stored result instantly and costs nothing on camera. The trace then shows "hasil tersimpan (cached, no charge)": that is honest, keep it visible or crop it, but never act out a wait.
3. Browser at 100% zoom, 1920x1080 or a phone-sized window centred, no other tabs, notifications off.

## In the edit

- Captions burned in, same words as the voice.
- Music 18 to 20 dB under the voice.
- The caption on the Jeejak card stays on screen for its whole duration.
- Check the export: duration 2:50 to 2:58, plays with sound, 1080p.

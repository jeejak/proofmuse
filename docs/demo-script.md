# Demo video script (2:56, hard maximum 3:00)

The submission asks for a **three-minute** demo video that highlights **only the features, code and functionality built during the hackathon**; judges must be able to tell what is new, or the entry can be disqualified. So:

- **Aim for 2:50 to 2:57, never above 3:00.** Never below 2:40.
- **Built from screenshots in promptmotion; no screen recording needed.** Every app shot is a real screenshot of the live demo (`cerebralvalley/VIDEO_ASSETS.md` maps each beat to its files), never a mock-up. Real app screens fill at least 1:40 (ProofMuse 0:30 to 2:01, then phone and console 2:01 to 2:19). Screenshots carry no address bar, so put the URL on screen as a caption on the first ProofMuse shot and the first console shot.
- **Jeejak and JejakPaket appear on the title card for under 15 seconds, captioned "Existing product, not part of this submission".** JejakPaket and the console appear once more, in the 2:01 beat, only to show what was built this week (ProofMuse as the console's check backend, PR #391 merged 2026-10-27, disclosed in `HACKATHON.md`), with the caption on screen for the whole beat. No other app or console shots.
- **The page opens in English** (EN | ID switch in the header). Flip to ID once, for a second, to show what the packer reads.
- **Speak, and caption the same words.** About 2.3 words a second, unhurried. Judges often watch muted.

Live URL: the deployed demo (see `docs/deploy.md`). Local fallback: `go run ./cmd/server`, then http://localhost:8080.

| Time | On screen | Say |
|---|---|---|
| 0:00 to 0:16 | A dark packing photo, then a blurry one, then the text "Dispute lost. The parcel is gone." | "Indonesian online sellers lose Shopee and TikTok Shop disputes because their packing photo turns out useless: too dark, label cut off, box never shown sealed. They find out days later, when nothing can be re-shot." |
| 0:16 to 0:30 | Title card: Jeejak logo, "JejakPaket, our packing-evidence app (existing product, not part of this submission)", then the ProofMuse title with "Built this week on Meta Muse Spark and SAM 3.1" | "We're Jeejak, from Kediri. Our app JejakPaket already records packing evidence. This week we built ProofMuse on Meta's models: it checks that evidence while the parcel is still on the table." |
| 0:30 to 0:54 | Caption `proofmuse.jeejak.id`. Sample **Too dark, then retaken**. Press **Check evidence**. Result: Weak evidence (Bukti lemah), "Photo too dark" with the retake instruction and the Indonesian line the packer sees; the green SAM label boxes on the photos | "A seller packs a mug and takes three photos. ProofMuse answers in about five seconds: Bukti lemah, weak evidence. The photos are too dark. It says why, and what to do, in Indonesian. The green boxes are SAM 3.1 finding the shipping label on each photo." |
| 0:54 to 1:10 | Press **Sample: use the retaken photos**, then **Check again**. Strong evidence; the tracking number under each photo; **Attempts**: #1 Weak evidence, #2 Strong evidence | "The seller retakes in better light and checks again. Bukti kuat. The tracking number is read from the label and matches the order. The fix takes seconds at the bench, not an argument days later." |
| 1:10 to 1:32 | Sample **Wrong label on the box**, **Check evidence**, Weak evidence "Tracking number on the label differs from the order". Open **Under the hood**, highlight the two **verify … confirmed: both reads agree** rows | "Here the wrong label went on the box. The model is never told the expected number; it only reads. Before ProofMuse may say wrong label, it reads that label again from a high-resolution SAM crop. Both reads agree, so the mismatch stands." |
| 1:32 to 1:49 | Back on the first sample's retaken set. Section 4, tap **kak paketnya kosong** (buyers write in Indonesian), **Read complaint**. "Box arrived empty", photos highlighted as answering it, the gap "a packing video is stronger" | "When a buyer complains, the seller pastes the message. Muse Spark only classifies it: empty box. Our rules pick the photos that answer it, and say honestly what photos cannot show." |
| 1:49 to 2:01 | Press **Download evidence packet**. The PDF opens: verdict banner, the claim section, photos with the label boxes, the SHA-256 line under each photo | "Then one button turns the checked photos into an evidence packet: verdict, label boxes and a fingerprint of every photo, ready to attach to the dispute." |
| 2:01 to 2:19 | **One real parcel through the whole path**, from the iPhone run (`screenshots/real-path/`, resi CGK2610270005): JejakPaket Demo scanning the demo waybill, the packing photos, the drop scan, then the Jeejak console (caption `console-demo.jeejak.id`) showing that same resi first in Drop evidence with Strong evidence, then its verdict panel (device iPhone 14 Pro). Caption for the whole beat: "JejakPaket and the console: existing products. Built this week: ProofMuse inside the console, demo only", plus a small "Staged photos, dummy label" on the phone shots | "And it already runs inside our console. The packer scans and photographs in our JejakPaket app. Once it syncs, the console sends the photos to ProofMuse, and the verdict appears next to the parcel. Demo only for now." |
| 2:19 to 2:45 | The README results table with "synthetic photos" on screen, then the line "Without verify: one misread became a false wrong-label" | "We measured it on a held-out test set of 62 parcels, labelled blind by a person. Every weak verdict was right and every weak parcel was caught, with no false alarms on strong evidence, at under one and a half US cents a parcel. Without the verify step, a one-digit misread became a false wrong-label." |
| 2:45 to 2:56 | End card: the pipeline in one line (Muse Spark observe, SAM 3.1 ground, reread, verify, Go rules), then ProofMuse by Jeejak, github.com/jeejak/proofmuse, the live URL, jeejak.id | "Muse Spark observes, SAM grounds, and plain Go rules decide. It is advice, not proof: the marketplace still decides. ProofMuse, by Jeejak." |

The voice is 361 words, about 2:37 of speech at 2.3 words a second; every slot is at least as long as its line. The rest is clicks, results held on screen and the end card. If the edit runs over 2:58, trim holds first, never the measured numbers, the verify rows or the console caption. Label any sped-up wait "sped up".

## Words to keep exact

Each is checked against `docs/eval/full-test.md` and `docs/eval/spark-only-test.md`. Do not round or rephrase them on screen or in the voice.

- **"62 parcels, held-out test set, labelled blind by a person."** The photos are AI-generated synthetic images with dummy waybills: say "synthetic" on screen in the results shot.
- **"Every weak verdict was right and every weak parcel was caught"** = 100% precision (21/21) and 100% recall (21/21) on Bukti lemah. **"No false alarms on strong evidence"** = 0 of 25.
- **"Under one and a half US cents a parcel"** = $0.0138 (1.4 cents). Latency is 4.6 s median; say "about five seconds".
- **"Without the verify step, a one-digit misread became a false wrong-label"**: Muse Spark alone flagged R49 as label_mismatch on the test split (95.5% lemah precision). Say "one", not "several".
- **Never "win the dispute".** It checks whether the evidence would hold up; the marketplace decides.
- **Model names:** Muse Spark 1.3 and SAM 3.1, through the Meta Model API.

## Before taking screenshots

1. `/health` on the live URL answers `ok`. The demo console is signed in as the demo owner, English selected, the help bubble closed.
2. Run every ProofMuse sample above once first, so each shot shows a stored result. The trace then reads "hasil tersimpan (cached, no charge)": that is honest, keep it or crop it, but never caption it as a live wait.
3. Desktop shots at 1440 px wide (2x), phone shots from the phone itself. No other tabs, no notifications, no real seller data anywhere in frame.
4. **The 2:01 beat must show the parcel the phone actually packed.** Done on 2026-10-27: the iPhone run of CGK2610270005 synced, and the console's check job sent it to ProofMuse on its own (Bukti kuat, label read CGK2610270005). The phone photographed staged label images, so keep the "Staged photos, dummy label" caption. Do not swap in the loader parcels (JP2000xxx, demo-device-2) as if a phone took them.

## In the edit

- Captions burned in, same words as the voice.
- Music 18 to 20 dB under the voice.
- The caption on the Jeejak card stays on screen for its whole duration.
- Pan and zoom the screenshots to the part the voice is talking about; hold each verdict on screen at least 2 seconds.
- Check the export: duration 2:50 to 2:58, plays with sound, 1080p.

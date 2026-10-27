package evidence

// Message is the fixed seller-facing text for a reason code: what is
// wrong, and what the packer should do about it while the parcel is
// still on the table. The model's own words never reach a seller.
type Message struct {
	Problem string `json:"problem"`
	Retake  string `json:"retake"`
	English string `json:"english"`
}

// Messages are keyed by reason code. Problem strings are ParcelProof's,
// unchanged; Retake strings are new in ProofMuse.
var Messages = map[string]Message{
	ReasonImageDark: {
		Problem: "Foto terlalu gelap",
		Retake:  "Foto ulang di tempat yang lebih terang, tanpa flash yang memantul.",
		English: "Photo too dark. Retake in brighter light.",
	},
	ReasonImageBlurry: {
		Problem: "Foto buram",
		Retake:  "Tahan HP diam sebentar lalu foto ulang, ketuk layar agar fokus.",
		English: "Photo blurry. Hold the phone still and retake.",
	},
	ReasonImageObstructed: {
		Problem: "Ada yang menutupi barang, label, atau segel",
		Retake:  "Singkirkan tangan atau benda yang menutupi, lalu foto ulang.",
		English: "Something covers the item, label or seal. Clear it and retake.",
	},
	ReasonLabelNotVisible: {
		Problem: "Label resi tidak terlihat",
		Retake:  "Tambah satu foto label resi dari dekat, seluruh label masuk bingkai.",
		English: "Shipping label not visible. Add a close-up of the whole label.",
	},
	ReasonLabelUnreadable: {
		Problem: "Nomor resi di label tidak terbaca",
		Retake:  "Foto label resi dari dekat, lurus dan tidak silau, sampai nomornya jelas.",
		English: "Tracking number unreadable. Retake the label close up, straight on, no glare.",
	},
	ReasonLabelMismatch: {
		Problem: "Nomor resi di label beda dengan pesanan",
		Retake:  "Cek lagi: pastikan label yang ditempel milik pesanan ini sebelum paket dikirim.",
		English: "Label tracking number differs from the order. Check the label belongs to this order.",
	},
	ReasonItemNotVisible: {
		Problem: "Barang tidak terlihat sebelum dikemas",
		Retake:  "Kalau kardus belum ditutup, foto barang di dalam kardus yang masih terbuka.",
		English: "Item not shown before packing. If the box is still open, photograph the item inside.",
	},
	ReasonNotSealed: {
		Problem: "Paket belum terlihat tersegel",
		Retake:  "Foto kardus setelah dilakban rapat, semua sisi atas terlihat.",
		English: "Parcel not shown sealed. Photograph the taped box.",
	},
	ReasonKoliMismatch: {
		Problem: "Jumlah koli di foto tidak sesuai",
		Retake:  "Foto semua koli pesanan ini bersama dalam satu bingkai.",
		English: "Parcel count in the photos differs from the order. Photograph all parcels together.",
	},
	ReasonItemMismatch: {
		Problem: "Barang di foto tampak beda dengan pesanan",
		Retake:  "Cek lagi barangnya sesuai pesanan sebelum kardus ditutup.",
		English: "Item looks different from the order listing. Check before closing the box.",
	},
	ReasonResiConflict: {
		Problem: "Ada lebih dari satu nomor resi di foto",
		Retake:  "Pastikan hanya label pesanan ini yang terlihat, lalu foto ulang labelnya.",
		English: "More than one tracking number appears. Show only this order's label.",
	},
}

// VerdictText is the badge text per verdict.
var VerdictText = map[string][2]string{
	VerdictKuat:       {"Bukti kuat", "strong evidence"},
	VerdictPerluDicek: {"Perlu dicek", "worth a look"},
	VerdictLemah:      {"Bukti lemah", "weak evidence"},
}

// Advisory is shown with every verdict. ProofMuse flags evidence
// quality; it never proves what was in the box or decides a dispute.
const Advisory = "Ini saran, bukan bukti. Hasil sengketa tetap diputuskan marketplace."

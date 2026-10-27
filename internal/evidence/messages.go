package evidence

// Message is the fixed text for a reason code: what is wrong, and what
// the packer should do about it while the parcel is still on the table.
// Sellers see the Indonesian; the English is for everyone else (and the
// demo). The model's own words never reach a seller.
type Message struct {
	Problem   string `json:"problem"`
	Retake    string `json:"retake"`
	ProblemEN string `json:"problem_en"`
	RetakeEN  string `json:"retake_en"`
}

// Messages are keyed by reason code. The nine Indonesian problem strings
// are ParcelProof's, unchanged; everything else is new in ProofMuse.
var Messages = map[string]Message{
	ReasonImageDark: {
		Problem: "Foto terlalu gelap", Retake: "Foto ulang di tempat yang lebih terang, tanpa flash yang memantul.",
		ProblemEN: "Photo too dark", RetakeEN: "Retake in brighter light, without a flash that glares.",
	},
	ReasonImageBlurry: {
		Problem: "Foto buram", Retake: "Tahan HP diam sebentar lalu foto ulang, ketuk layar agar fokus.",
		ProblemEN: "Photo blurry", RetakeEN: "Hold the phone still for a moment and retake; tap the screen to focus.",
	},
	ReasonImageObstructed: {
		Problem: "Ada yang menutupi barang, label, atau segel", Retake: "Singkirkan tangan atau benda yang menutupi, lalu foto ulang.",
		ProblemEN: "Something covers the item, label or seal", RetakeEN: "Move the hand or object out of the way and retake.",
	},
	ReasonLabelNotVisible: {
		Problem: "Label resi tidak terlihat", Retake: "Tambah satu foto label resi dari dekat, seluruh label masuk bingkai.",
		ProblemEN: "Shipping label not visible", RetakeEN: "Add a close-up of the shipping label with the whole label in frame.",
	},
	ReasonLabelUnreadable: {
		Problem: "Nomor resi di label tidak terbaca", Retake: "Foto label resi dari dekat, lurus dan tidak silau, sampai nomornya jelas.",
		ProblemEN: "Tracking number on the label unreadable", RetakeEN: "Photograph the label close up, straight on and without glare, until the number is clear.",
	},
	ReasonLabelMismatch: {
		Problem: "Nomor resi di label beda dengan pesanan", Retake: "Cek lagi: pastikan label yang ditempel milik pesanan ini sebelum paket dikirim.",
		ProblemEN: "Tracking number on the label differs from the order", RetakeEN: "Check that the label on the box belongs to this order before it ships.",
	},
	ReasonItemNotVisible: {
		Problem: "Barang tidak terlihat sebelum dikemas", Retake: "Kalau kardus belum ditutup, foto barang di dalam kardus yang masih terbuka.",
		ProblemEN: "Item not shown before packing", RetakeEN: "If the box is still open, photograph the item inside it.",
	},
	ReasonNotSealed: {
		Problem: "Paket belum terlihat tersegel", Retake: "Foto kardus setelah dilakban rapat, semua sisi atas terlihat.",
		ProblemEN: "Parcel not shown sealed", RetakeEN: "Photograph the box after taping it shut, with the whole top visible.",
	},
	ReasonKoliMismatch: {
		Problem: "Jumlah koli di foto tidak sesuai", Retake: "Foto semua koli pesanan ini bersama dalam satu bingkai.",
		ProblemEN: "Parcel count in the photos differs from the order", RetakeEN: "Photograph all of this order's parcels together in one frame.",
	},
	ReasonItemMismatch: {
		Problem: "Barang di foto tampak beda dengan pesanan", Retake: "Cek lagi barangnya sesuai pesanan sebelum kardus ditutup.",
		ProblemEN: "Item looks different from the order", RetakeEN: "Check the item matches the order before closing the box.",
	},
	ReasonResiConflict: {
		Problem: "Ada lebih dari satu nomor resi di foto", Retake: "Pastikan hanya label pesanan ini yang terlihat, lalu foto ulang labelnya.",
		ProblemEN: "More than one tracking number in the photos", RetakeEN: "Make sure only this order's label is visible, then retake the label.",
	},
}

// VerdictText is the badge text per verdict: Indonesian, then English.
var VerdictText = map[string][2]string{
	VerdictKuat:       {"Bukti kuat", "Strong evidence"},
	VerdictPerluDicek: {"Perlu dicek", "Worth a look"},
	VerdictLemah:      {"Bukti lemah", "Weak evidence"},
}

// Advisory is shown with every verdict. ProofMuse flags evidence
// quality; it never proves what was in the box or decides a dispute.
const (
	Advisory   = "Ini saran, bukan bukti. Hasil sengketa tetap diputuskan marketplace."
	AdvisoryEN = "This is advice, not proof. The marketplace still decides any dispute."
)

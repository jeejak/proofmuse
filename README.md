# ProofMuse by Jeejak

**Will this packing photo hold up in a dispute? Find out while the parcel is still on the table.**

ProofMuse is the successor to [ParcelProof](https://github.com/jeejak/parcel-proof), built on Meta Muse Spark 1.3 and SAM 3.1. An Indonesian online seller photographs the packing, and within seconds gets practical feedback in Indonesian: *Foto label kurang jelas, silakan foto ulang.* The models observe; a Go rules engine decides. The result is advisory. It flags evidence-quality issues for a person to act on and never claims to prove what was in the box or to guarantee a dispute outcome.

Built for the Meta Global AI Developer Hackathon (Meta x Cerebral Valley, 2026), Startup track. See [HACKATHON.md](HACKATHON.md) for what was built during the event and what was ported.

## Status

Day 1. The server answers `/health`; the pipeline follows commit by commit.

## Run locally

```bash
cp .env.example .env    # add MODEL_API_KEY
go run ./cmd/server     # http://localhost:8080/health
```

## License

Apache-2.0. Copyright 2026 CV. Adisatya IT Consultant. Jeejak, JejakPaket and ParcelProof are its trademarks.

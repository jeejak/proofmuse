"""Check byte-stable SYNTHETIC metadata rebuild and frozen-asset replay."""
import contextlib
import io
import json
import tempfile
from pathlib import Path
from prepare import ROOT,WORK,sha
from finalize import main as finalize
from replay_assets import restore

def main():
    names=['README.md','labels.csv','photos.csv','orders.json','claims.csv','retake_pairs.csv','split.csv',
           'review/index.html','review/notes.md','review/uncertain-labels.csv','review/verification.json']
    before={n:sha(ROOT/n) for n in names}
    with contextlib.redirect_stdout(io.StringIO()):finalize()
    assert before=={n:sha(ROOT/n) for n in names},'Metadata rebuild changed frozen outputs'
    prov=[json.loads(x) for x in (ROOT/'PROVENANCE.jsonl').read_text().splitlines()]
    with tempfile.TemporaryDirectory() as folder:
        base=Path(folder)
        for rid,photo in [('R32','02.jpg'),('R48','01.jpg')]:
            source=WORK/'jeejak-eval-set-synthetic'/rid/photo;dest=base/rid/photo
            restore(source,dest,sha(ROOT/'records'/rid/photo))
        for rid,photo in [('R65','01.jpg'),('R86','03.jpg'),('R101','03.jpg'),('R87','02.jpg')]:
            row=next(r for r in prov if r['record_id']==rid and r['photo']==photo)
            restore(Path(row['source_path']),base/rid/photo,row['sha256'])
    print('PASS: synthetic metadata byte-stable rebuild and six frozen-image replay checks.')

if __name__=='__main__':main()

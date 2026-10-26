"""Repair contact-like entropy tokens in SYNTHETIC JPEG copies, preserving decoded pixels."""
import json
from PIL import Image
from prepare import ROOT,WORK,save_jpeg
from ingest_image import ingest
from verify import EMAIL,PHONE

def main():
    provenance=[json.loads(s) for s in (ROOT/'PROVENANCE.jsonl').read_text().splitlines()]
    for p in sorted((ROOT/'records').glob('*/*.jpg')):
        raw=p.read_bytes()
        if not (EMAIL.search(raw) or PHONE.search(raw)):continue
        with Image.open(p) as old:before=old.tobytes()
        if int(p.parent.name[1:])<65:
            source=WORK/'jeejak-eval-set-synthetic'/p.parent.name/p.name
            with Image.open(source) as im:
                im.thumbnail((1280,1280),Image.Resampling.LANCZOS);save_jpeg(im,p)
        else:
            job=next(r.copy() for r in provenance if r['record_id']==p.parent.name and r['photo']==p.name)
            for f in ('sha256','source_sha256','generator','generated_on','transform'):job.pop(f,None)
            ingest(job)
        with Image.open(p) as new:assert before==new.tobytes(),'Decoded pixels changed: '+str(p)
        print('Synthetic JPEG compression changed with equivalent decoded pixels: '+p.parent.name+'/'+p.name)

if __name__=='__main__':main()

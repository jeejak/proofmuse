"""Copy a built-in generated synthetic asset, resize, and record provenance. No API."""
import argparse
import json
from pathlib import Path
from PIL import Image
from prepare import ROOT, sha, save_jpeg

def ingest(job):
    dst=ROOT/'records'/job['record_id']/job['photo']
    dst.parent.mkdir(exist_ok=True)
    src=Path(job['source_path'])
    with Image.open(src) as im:
        im.thumbnail((1280,1280),Image.Resampling.LANCZOS)
        save_jpeg(im,dst)
    log=ROOT/'PROVENANCE.jsonl'
    rows=[json.loads(s) for s in log.read_text().splitlines()] if log.exists() else []
    key=(job['record_id'],job['photo'])
    rows=[r for r in rows if (r['record_id'],r['photo'])!=key]
    job=dict(job,synthetic=True)
    rows.append(dict(**job,sha256=sha(dst),source_sha256=sha(src),
                     generated_on='2026-10-26',
                     transform='JPEG quality 95, chroma 4:4:4, max side 1280, metadata stripped, deterministic contact-token-free entropy encoding'))
    rows.sort(key=lambda r:(r['record_id'],r['photo']))
    log.write_text(''.join(json.dumps(r,ensure_ascii=False)+'\n' for r in rows))
    print(str(dst))

if __name__=='__main__':
    p=argparse.ArgumentParser()
    p.add_argument('job',help='JSON file containing prompt, source_path, record_id, photo')
    a=p.parse_args()
    ingest(json.loads(Path(a.job).read_text()))

"""Deterministically restore SYNTHETIC JPEGs from frozen local source assets, no APIs."""
import json
from PIL import Image
from prepare import ROOT,WORK,sha,save_jpeg

def restore(source,target,expected=None):
    if not source.exists():raise FileNotFoundError('Frozen source asset missing: '+str(source))
    if target.exists():
        if expected and sha(target)!=expected:raise ValueError('Existing output hash differs: '+str(target))
        return
    target.parent.mkdir(parents=True,exist_ok=True)
    with Image.open(source) as im:
        im.thumbnail((1280,1280),Image.Resampling.LANCZOS)
        save_jpeg(im,target)
    if expected and sha(target)!=expected:raise ValueError('Replay differs; check pinned Pillow version: '+str(target))

def main():
    from pathlib import Path
    for source in sorted((WORK/'jeejak-eval-set-synthetic').glob('R*/*.jpg')):
        restore(source,ROOT/'records'/source.parent.name/source.name)
    for line in (ROOT/'PROVENANCE.jsonl').read_text().splitlines():
        p=json.loads(line);restore(Path(p['source_path']),ROOT/'records'/p['record_id']/p['photo'],p['sha256'])
    print('Synthetic asset replay complete. Existing verified JPEGs retained.')

if __name__=='__main__':main()

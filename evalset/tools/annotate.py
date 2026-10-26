"""Persist pixel-inspected model proposals. Never marks a human review."""
import json,sys
from prepare import ROOT,dump

def add(rows):
    dest=ROOT/'tools/inspected-proposals.json'
    existing=json.loads(dest.read_text()) if dest.exists() else {}
    for row in rows:
        key,present,readable,issues,boxes,sealed,item,box,*note=row
        existing[key]=dict(label_in_photo=present,label_readable=readable,image_quality_issues=issues,
                           boxes_in_photo=boxes,sealed_box_in_photo=sealed,item_in_photo=item,label_box=box,
                           note=note[0] if note else '',pixel_inspected=True,synthetic=True)
    dump(dest,dict(sorted(existing.items())))

if __name__=='__main__': add(json.loads(sys.argv[1]))

"""Finalize and verify frozen SYNTHETIC fixtures without model or network calls."""
import json
from prepare import ROOT,csvwrite,dump
from build_metadata import build_photos,main as build_metadata
from reconcile_new import main as reconcile
from verify import check

def main():
    build_photos()
    reconcile()
    build_metadata()
    counts=check()
    dump(ROOT/'review/verification.json',counts)
    photos=__import__('csv').DictReader((ROOT/'photos.csv').open())
    uncertain=[]
    for p in photos:
        for f in ('label_in_photo','label_readable','sealed_box_in_photo','item_in_photo'):
            if p[f]=='uncertain':uncertain.append(dict(record_id=p['record_id'],photo=p['photo'],field=f,value='uncertain'))
    csvwrite(ROOT/'review/uncertain-labels.csv',uncertain,['record_id','photo','field','value'])
    path=ROOT/'README.md';text=path.read_text();marker='<!-- SYNTHETIC-COUNTS -->'
    text=text.split(marker)[0].rstrip()+'\n\n'+marker+'\n```json\n'+json.dumps(counts,indent=2)+'\n```\n\nPASS: synthetic fixture checks. This is not a real-photo accuracy result.\n'
    path.write_text(text)

if __name__=='__main__':main()

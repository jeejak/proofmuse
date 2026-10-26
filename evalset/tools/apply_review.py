"""Merge explicit human decisions into this SYNTHETIC set. Does not call any API."""
import argparse
import csv
from datetime import datetime,timedelta
from pathlib import Path
import os
import tempfile
from prepare import ROOT,csvwrite
from build_metadata import PHOTO_FIELDS,readcsv

PHOTO_LABELS=PHOTO_FIELDS[3:10]
RECORD_LABELS=['image_usable','label_visible','label_resi_read','package_sealed','item_visible','koli_seen','human_verdict']

def timestamp(value):
    dt=datetime.fromisoformat(value)
    if dt.utcoffset()!=timedelta(hours=7):raise ValueError('Review time must have WIB offset +07:00')

def validate_photo(row):
    for k in ('label_in_photo','label_readable','sealed_box_in_photo','item_in_photo'):
        if row[k] not in ('yes','no','uncertain'):raise ValueError('Invalid '+k)
    if not row['boxes_in_photo'].isdigit():raise ValueError('Invalid boxes_in_photo')
    box=row['label_box']
    if box:
        a=list(map(int,box.split()))
        if len(a)!=4 or not (0<=a[0]<a[2]<=1000 and 0<=a[1]<a[3]<=1000):raise ValueError('Invalid label box')
    if row['label_in_photo']=='yes' and not box:raise ValueError('Visible label requires label_box')
    if row['label_in_photo']=='no' and box:raise ValueError('Absent label cannot have label_box')
    if row['label_readable']=='yes' and row['label_in_photo']!='yes':raise ValueError('Readable label must be visible')
    if not set(filter(None,row['image_quality_issues'].split('|'))) <= {'blur','glare','too_dark','too_far','cropped','obstructed'}:raise ValueError('Invalid quality issue')

def merge(root,decisions_path):
    root=Path(root)
    def read(name):return list(csv.DictReader((root/name).open()))
    photos=read('photos.csv');labels=read('labels.csv')
    by={(p['record_id'],p['photo']):p for p in photos}
    byrec={r['record_id']:r for r in labels}
    decisions=list(csv.DictReader(Path(decisions_path).open()))
    seen=set();record_updates={};touched=0
    for d in decisions:
        key=(d.get('record_id'),d.get('photo'))
        if key not in by or key in seen:raise ValueError('Unknown or duplicate review photo: '+str(key))
        seen.add(key)
        if d.get('photo_review')=='yes':
            if not d.get('reviewed_by','').strip():raise ValueError('Reviewer name is required')
            timestamp(d['reviewed_at']);validate_photo(d)
            by[key].update({f:d[f] for f in PHOTO_LABELS})
            by[key].update(needs_review='no',reviewed_by=d['reviewed_by'],reviewed_at=d['reviewed_at'])
            touched+=1
        if d.get('record_review')=='yes':
            if not d.get('record_reviewed_by','').strip():raise ValueError('Record reviewer is required')
            timestamp(d['record_reviewed_at'])
            update={f:d['record_'+f] for f in RECORD_LABELS}
            if update['image_usable'] not in ('ok','dark','blurry','obstructed'):raise ValueError('Invalid image_usable')
            for f in ('label_visible','package_sealed','item_visible'):
                if update[f] not in ('yes','no','unsure'):raise ValueError('Invalid record '+f)
            if update['human_verdict'] not in ('kuat','perlu_dicek','lemah'):raise ValueError('Invalid verdict')
            if update['koli_seen'] and not update['koli_seen'].isdigit():raise ValueError('Invalid koli_seen')
            from verify import PATTERNS
            import re
            if update['label_resi_read'] and not re.fullmatch(PATTERNS[byrec[key[0]]['courier']],update['label_resi_read']):raise ValueError('Record resi does not match courier format')
            update.update(record_fields_source='human',record_reviewed_by=d['record_reviewed_by'],record_reviewed_at=d['record_reviewed_at'])
            old=record_updates.get(key[0])
            if old and old!=update:raise ValueError('Conflicting record reviews: '+key[0])
            record_updates[key[0]]=update
    for r in labels:
        record_photos=[p for p in photos if p['record_id']==r['record_id']]
        r['label_source']='human' if all(p['needs_review']=='no' for p in record_photos) else 'proposed'
        r.setdefault('record_fields_source','proposed');r.setdefault('record_reviewed_by','');r.setdefault('record_reviewed_at','')
        if r['record_id'] in record_updates:r.update(record_updates[r['record_id']])
    # Validate all decisions before replacing either file. Keep per-file replacements atomic.
    for name,rows,fields in [('photos.csv',photos,list(photos[0])),('labels.csv',labels,list(labels[0]))]:
        fd,tmp=tempfile.mkstemp(prefix=name+'.',dir=root);os.close(fd)
        try:csvwrite(Path(tmp),rows,fields);os.replace(tmp,root/name)
        finally:
            if Path(tmp).exists():Path(tmp).unlink()
    print(f'Applied {touched} explicit photo decisions and {len(record_updates)} record reviews to synthetic data.')

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('decisions',type=Path);a=p.parse_args()
    merge(ROOT,a.decisions)
    from build_review import build
    build()

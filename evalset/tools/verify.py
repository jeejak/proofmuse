"""Verify SYNTHETIC fixture integrity, provenance, review boundaries and text privacy."""
import csv
import json
import re
import sys
from collections import Counter,defaultdict
from pathlib import Path
from PIL import Image
from prepare import ROOT,WORK,sha

PATTERNS={'SPX Express':r'SPXID\d{10}','J&T Express':r'JP\d{10}','JNE':r'16\d{12}',
          'SiCepat':r'00\d{10}','AnterAja':r'1000\d{11}','Ninja Xpress':r'NLIDAP\d{8}'}
# Token boundaries avoid confusing a digit substring inside a courier resi with a phone.
PHONE=re.compile(rb'(?<![A-Za-z0-9])(?:08[0-9]{8,12}|\+62[0-9]{8,12})(?![0-9])')
EMAIL=re.compile(rb'[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}')

def check(root=ROOT,check_sources=True):
    root=Path(root);errors=[]
    def require(ok,message):
        if not ok:errors.append(message)
    def read(name):return list(csv.DictReader((root/name).open()))
    labels=read('labels.csv');photos=read('photos.csv');splits=read('split.csv');claims=read('claims.csv')
    orders=json.loads((root/'orders.json').read_text());pairs=read('retake_pairs.csv')
    ridset={r['record_id'] for r in labels}
    require(len(labels)==104 and ridset=={f'R{n:02d}' for n in range(1,105)},'Expected exactly R01 to R104')
    require(len(ridset)==len(labels),'Duplicate labels')
    files={f'{p.parent.name}/{p.name}':p for p in (root/'records').glob('*/*.jpg')}
    pkeys=[f'{p["record_id"]}/{p["photo"]}' for p in photos]
    require(len(pkeys)==len(set(pkeys)),'Duplicate photo rows')
    require(set(pkeys)==set(files),'Photos on disk and photos.csv differ')
    require(len(files)==399,'Expected 279 pre-existing plus 120 new photos')
    require({r['record_id'] for r in photos}==ridset,'Photo record coverage mismatch')
    require(len(orders)==len(ridset) and {o['order_id'] for o in orders}=={'SYN-'+rid for rid in ridset},'Order coverage mismatch')
    require(len(splits)==len(ridset) and {s['record_id'] for s in splits}==ridset,'Split coverage mismatch')
    split={s['record_id']:s['split'] for s in splits}
    require(set(split.values())=={'dev','test'},'Split values must be dev or test')
    require(.35<=sum(s=='dev' for s in split.values())/len(split)<=.45,'Dev fraction not about 40 percent')
    byrec={r['record_id']:r for r in labels};pby=defaultdict(list)
    if check_sources:
        originals=list(csv.DictReader((WORK/'jeejak-eval-set-synthetic/labels.csv').open()))
        for original in originals:
            current=byrec[original['record_id']]
            if current.get('record_fields_source') not in ('human','derived_from_human_photos'):
                require(all(current[k]==v for k,v in original.items()),original['record_id']+' original label columns changed without explicit human record review')
    from derive_records import derive,FIELDS as DERIVED_FIELDS
    for fresh in derive([r for r in labels if r.get('record_fields_source')=='derived_from_human_photos'],photos):
        cur=byrec[fresh['record_id']]
        require(all(cur[k]==fresh[k] for k in DERIVED_FIELDS+['verdict_reasons']),fresh['record_id']+' derived record fields out of date; run tools/derive_records.py')
    for p in photos:
        pby[p['record_id']].append(p)
        key=p['record_id']+'/'+p['photo']
        for f in ['label_in_photo','label_readable','sealed_box_in_photo','item_in_photo']:
            require(p[f] in ('yes','no','uncertain'),key+' invalid '+f)
        require(p['boxes_in_photo'].isdigit(),key+' invalid count')
        require(set(filter(None,p['image_quality_issues'].split('|'))) <= {'blur','glare','too_dark','too_far','cropped','obstructed'},key+' invalid issue')
        if p['label_box']:
            try:
                a=list(map(int,p['label_box'].split()))
                require(len(a)==4 and 0<=a[0]<a[2]<=1000 and 0<=a[1]<a[3]<=1000,key+' invalid label_box')
            except (ValueError,IndexError):require(False,key+' malformed label_box')
        require(not(p['label_in_photo']=='no' and p['label_box']),key+' box for absent label')
        require(not(p['label_in_photo']=='yes' and not p['label_box']),key+' missing visible label box')
        require(p['label_readable']!='yes' or p['label_in_photo']=='yes',key+' readable absent label')
        require(p['needs_review'] in ('yes','no'),key+' invalid review state')
        if p['needs_review']=='no':require(bool(p['reviewed_by'] and p['reviewed_at']),key+' lacks human reviewer')
        else:require(not p['reviewed_by'] and not p['reviewed_at'],key+' proposal stamped as reviewed')
        if key in files:
            with Image.open(files[key]) as im:
                require(im.format=='JPEG' and max(im.size)==1280,key+' invalid format or size')
                orientation='portrait' if im.height>im.width else 'landscape'
                require(p['orientation']==orientation,key+' incorrect orientation')
    for r in labels:
        rid=r['record_id']
        require(r['courier'] in PATTERNS,rid+' unknown courier')
        for f in ['resi','label_resi_read']:
            require(not r[f] or bool(re.fullmatch(PATTERNS[r['courier']],r[f])),rid+' invalid '+f)
        require(r['human_verdict'] in ('kuat','perlu_dicek','lemah'),rid+' invalid verdict')
        allreviewed=bool(pby[rid]) and all(p['needs_review']=='no' for p in pby[rid])
        require(r['label_source']==('human' if allreviewed else 'proposed'),rid+' incorrect label_source')
        if int(rid[1:])>=65:require(len(pby[rid]) in (3,4),rid+' needs 3 or 4 photos')
    for o in orders:
        r=byrec[o['order_id'][4:]]
        require(o['resi']==r['resi'] and o['koli']==int(r['koli']) and o['courier']==r['courier'],o['order_id']+' order/record mismatch')
        require(o['listing_item']==r['listing_item'] and o.get('synthetic') is True,o['order_id']+' invalid mock order')
    require(Counter(o['marketplace'] for o in orders)==Counter(shopee=62,tiktok=42),'Marketplace distribution mismatch')
    require(len(pairs)==8,'Expected eight retake pairs')
    for p in pairs:
        b,g=p['bad_record'],p['good_record']
        require(split[b]==split[g],p['pair_id']+' retake leakage')
        require(byrec[g]['retake_of']==b and byrec[b]['resi']==byrec[g]['resi']==p['resi'],p['pair_id']+' pair identity mismatch')
    portraits=[r for r in labels if r['group']=='portrait']
    require(Counter(r['courier'] for r in portraits)==Counter({'SPX Express':2,'J&T Express':2,'JNE':2}),'Portrait courier balance mismatch')
    require(all(p['orientation']=='portrait' for r in portraits for p in pby[r['record_id']]),'Portrait group contains landscape photo')
    for field in ['group','human_verdict']:
        for v in {r[field] for r in labels}:
            require({split[r['record_id']] for r in labels if r[field]==v}=={'dev','test'},field+' '+v+' not in both splits')
    types={'barang_kurang','barang_rusak','paket_kosong','salah_kirim','tidak_diterima'}
    require(Counter(c['claim_type'] for c in claims)==Counter({t:8 for t in types}),'Expected eight claims per type')
    require(len({c['claim_id'] for c in claims})==40,'Duplicate claim ids')
    weak=0
    for c in claims:
        require(c['order_id'] in {o['order_id'] for o in orders},'Unknown claim order')
        rid=c['order_id'][4:]
        weak+=byrec[rid]['human_verdict']=='lemah'
        for key in filter(None,c['evidence_that_answers'].split('|')):
            require(key in files and key.startswith(rid+'/'),c['claim_id']+' references wrong evidence')
        if c['claim_type']=='tidak_diterima':require('delivery_tracking_missing' in c['evidence_missing'],c['claim_id']+' claims photos establish delivery')
    require(weak>=15,'Fewer than 15 claims reference weak evidence')
    provenance=[json.loads(line) for line in (root/'PROVENANCE.jsonl').read_text().splitlines()]
    newkeys={k for k in files if int(k.split('/')[0][1:])>=65}
    provkeys=[r['record_id']+'/'+r['photo'] for r in provenance]
    require(set(provkeys)==newkeys and len(provkeys)==len(newkeys),'New-image provenance mismatch')
    for r in provenance:
        key=r['record_id']+'/'+r['photo']
        require(r.get('synthetic') is True and bool(r.get('prompt')) and bool(r.get('source_path')),key+' incomplete provenance')
        require(key in files and sha(files[key])==r['sha256'],key+' provenance hash mismatch')
    hashes=[sha(p) for p in files.values()]
    require(len(hashes)==len(set(hashes)),'Duplicate image SHA256')
    for p in root.rglob('*'):
        if not p.is_file() or '__pycache__' in p.parts:continue
        raw=p.read_bytes()
        require(not PHONE.search(raw),'Phone-like token in '+str(p.relative_to(root)))
        require(not EMAIL.search(raw),'Email-like token in '+str(p.relative_to(root)))
        if p.suffix not in ('.jpg','.png','.pyc'):require(b'\xe2\x80\x94' not in raw,'Em dash in '+str(p.relative_to(root)))
    if check_sources:
        snapshot=json.loads((root/'tools/source-snapshot.json').read_text())
        for name,digest in snapshot.get('files',snapshot).items():require((WORK/name).exists() and sha(WORK/name)==digest,'Read-only source changed: '+name)
    counts={'synthetic':True,'records':len(labels),'photos':len(files),'pre_existing_photos':279,'new_photos':len(newkeys),
            'verdict':dict(sorted(Counter(r['human_verdict'] for r in labels).items())),
            'group':dict(sorted(Counter(r['group'] for r in labels).items())),
            'split':dict(sorted(Counter(split.values()).items())),
            'claim_type':dict(sorted(Counter(c['claim_type'] for c in claims).items())),
            'claims_on_weak_records':weak,'retake_pairs':len(pairs),
            'needs_review':sum(p['needs_review']=='yes' for p in photos)}
    print(json.dumps(counts,indent=2))
    if errors:
        for error in errors:print('FAIL: '+error,file=sys.stderr)
        raise ValueError(f'{len(errors)} synthetic fixture checks failed')
    print('PASS: synthetic fixture checks. This is not a real-photo accuracy result.')
    return counts

if __name__=='__main__':check()

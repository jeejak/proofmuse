"""Build deterministic SYNTHETIC fixtures from inspected proposals, never from model APIs."""
import csv
import json
import random
from collections import Counter,defaultdict
from PIL import Image
from prepare import ROOT,SEED,csvwrite,dump

PHOTO_FIELDS=['record_id','photo','orientation','label_in_photo','label_readable','image_quality_issues',
              'boxes_in_photo','sealed_box_in_photo','item_in_photo','label_box','needs_review','reviewed_by','reviewed_at']

def readcsv(name): return list(csv.DictReader((ROOT/name).open()))

def build_photos():
    inspected=json.loads((ROOT/'tools/inspected-proposals.json').read_text())
    previous={(r['record_id'],r['photo']):r for r in readcsv('photos.csv')} if (ROOT/'photos.csv').exists() else {}
    rows=[]
    for p in sorted((ROOT/'records').glob('*/*.jpg')):
        key=f'{p.parent.name}/{p.name}'
        if key not in inspected:
            raise ValueError(f'Not pixel inspected: {key}; inspect before building photos.csv')
        proposal=inspected[key]
        assert proposal['pixel_inspected']
        with Image.open(p) as im: orientation='landscape' if im.width>=im.height else 'portrait'
        row=dict(record_id=p.parent.name,photo=p.name,orientation=orientation,
                 **{k:proposal[k] for k in PHOTO_FIELDS[3:10]},needs_review='yes',reviewed_by='',reviewed_at='')
        old=previous.get((p.parent.name,p.name))
        if old and old.get('needs_review')=='no': row=old
        rows.append(row)
    csvwrite(ROOT/'photos.csv',rows,PHOTO_FIELDS)
    notes=['# Synthetic data: pixel inspection notes','',
           'All entries below are model proposals, not human grades. Bounding boxes are approximate axis-aligned boxes.',
           'When multiple labels are visible, label_box encloses the main or closest shipping label, not all labels.',
           'sealed_box_in_photo means at least one visibly taped-closed shipping box, not every box in a multi-box photo.',
           'boxes_in_photo counts distinct visible shipping boxes, including partially cropped boxes.','']
    for key,p in sorted(inspected.items()):
        unsure=[k for k,v in p.items() if v=='uncertain']
        if p.get('note') or unsure:
            notes.append(f'- {key}: '+ ('Uncertain '+', '.join(unsure)+'. ' if unsure else '')+p.get('note',''))
    (ROOT/'review/notes.md').write_text('\n'.join(notes)+'\n')
    return rows

def build_split(labels):
    # Group retakes, shared-resi records, and their original together to avoid leakage.
    parent={r['record_id']:r['retake_of'] or r['record_id'] for r in labels}
    def find(x):
        while parent[x]!=x: x=parent[x]
        return x
    units=defaultdict(list)
    for r in labels: units[find(r['record_id'])].append(r)
    units=list(units.values())
    groups=Counter(r['group'] for r in labels)
    verdicts=Counter(r['human_verdict'] for r in labels)
    rng=random.Random(SEED)
    target=round(len(labels)*.4)
    best=None
    for _ in range(4000):
        trial=units.copy();rng.shuffle(trial)
        dev=[]
        for u in trial:
            if len(dev)+len(u)<=target: dev.extend(u)
        gc=Counter(r['group'] for r in dev);vc=Counter(r['human_verdict'] for r in dev)
        if any(gc[g]==0 or gc[g]==n for g,n in groups.items()): continue
        if any(vc[v]==0 or vc[v]==n for v,n in verdicts.items()): continue
        score=sum(abs(gc[g]-.4*n) for g,n in groups.items())+sum(abs(vc[v]-.4*n) for v,n in verdicts.items())
        if best is None or score<best[0]: best=(score,{r['record_id'] for r in dev})
    assert best is not None,'Cannot satisfy stratification'
    csvwrite(ROOT/'split.csv',[dict(record_id=r['record_id'],split='dev' if r['record_id'] in best[1] else 'test') for r in labels])

CLAIMS={
 'barang_kurang':[
 'kak barangnya kurang satu, yg satunya mana ya?', 'gan cuma masuk satu barang, harusnya lebih 😕',
 'KOK ISINYA KURANG? tolong cek packingnya', 'udh dibuka kak, ada item yg ga ikut',
 'kak kayaknya ada yg ketinggalan pas dipacking deh', 'isinya ga lengkap nih, bisa dicekin?',
 'yg datang kurang kak... ada foto sebelum ditutup ga?', 'aku terima sebagian aja, sisanya blm ada'],
 'barang_rusak':[
 'kak barangnya nyampe rusak 😭', 'gan ini penyok pas aku buka, packingnya gimana?',
 'BARANG RUSAK KAK, minta dicek ya', 'udh sampe tp ada yg pecah, sedih bgt',
 'kak ada lecetnya, sebelum dikirim kondisinya gmn?', 'ini rusak di jalan apa dr awal ya kak?',
 'bungkusnya kebuka trus barangnya rusak nih', 'kak barangnya retak, ada foto pas dipacking?'],
 'paket_kosong':[
 'kak paketnya kosong, serius 😭', 'GAN INI CUMA KARDUS DOANG',
 'udh buka tp ga ada barangnya kak', 'kok isinya kertas aja ya? barangnya mana',
 'aku terima kotak kosong nih kak, tlg dicek', 'paket dateng kosong, ada bukti masukin barang ga?',
 'kak ga ada isi sama sekali pas dibuka', 'ini beneran dikirim barangnya? kotaknya kosong'],
 'salah_kirim':[
 'kak pesen mug merah kok dateng biru?', 'hijabnya ijo kak, aku pilih beige 😕',
 'aku pesen charger putih gan, yg dtg item', 'KAK SALAH WARNA, kotak pensilnya merah',
 'ini bukan pesenanku kayaknya kak, cek labelnya dong', 'gan barang ketuker ga sih? beda yg aku beli',
 'kak salah kirim nih, ada foto isinya sebelum packing?', 'labelnya kayak beda kak, tolong cocokkin pesenanku'],
 'tidak_diterima':[
 'kak paket blm aku terima, kok udh selesai?', 'gan di aplikasi terkirim tp ga ada paket',
 'AKU BELUM TERIMA KAK, cek ke kurir dong', 'udh nunggu seharian ga dateng2 😕',
 'kak statusnya delivered tp paketnya mana ya', 'ga ada yg nerima di sini kak, tlg bantu lacak',
 'ini bener udh diserahin ke kurir blm?', 'kak pesenanku blm sampe, ada bukti serah terimanya?']}
CLAIM_RECORDS={
 'barang_kurang':[14,20,26,42,43,55,7,95],
 'barang_rusak':[15,21,27,31,48,59,99,3],
 'paket_kosong':[16,22,28,42,44,45,46,1],
 'salah_kirim':[101,102,103,104,36,37,39,63],
 'tidak_diterima':[17,23,32,38,61,62,2,100]}

def build_claims(photos):
    by=defaultdict(list)
    for r in photos: by[r['record_id']].append(r)
    rows=[]
    for typ,texts in CLAIMS.items():
        for n,text in zip(CLAIM_RECORDS[typ],texts):
            rid=f'R{n:02d}';ps=by[rid]
            usable=lambda p:not set(p['image_quality_issues'].split('|'))&{'blur','too_dark','obstructed'}
            items=[p for p in ps if p['item_in_photo']=='yes' and usable(p)]
            seals=[p for p in ps if p['sealed_box_in_photo']=='yes' and usable(p)]
            labs=[p for p in ps if p['label_readable']=='yes' and usable(p)]
            gaps=[];answers=[]
            if typ in ('barang_kurang','barang_rusak','paket_kosong','salah_kirim'):
                answers.extend(items[:1])
                if not items:gaps.append('item_not_visible')
                answers.extend(seals[-1:])
                if not seals:gaps.append('not_sealed')
            if typ in ('salah_kirim','tidak_diterima'):
                answers.extend(labs[-1:])
                if not labs:gaps.append('readable_label_missing')
            if typ=='barang_kurang':gaps.append('item_quantity_not_established')
            if typ=='barang_rusak':gaps.append('delivery_condition_not_shown')
            if typ=='paket_kosong':gaps.append('continuous_packing_and_delivery_not_shown')
            if typ=='tidak_diterima':gaps.extend(['delivery_tracking_missing','handover_proof_missing'])
            unique=list(dict.fromkeys(f'{rid}/{p["photo"]}' for p in answers))
            rows.append(dict(claim_id=f'SYN-C{len(rows)+1:02d}',order_id='SYN-'+rid,text=text,claim_type=typ,
                             evidence_that_answers='|'.join(unique),evidence_missing='|'.join(gaps)))
    csvwrite(ROOT/'claims.csv',rows)

def main():
    labels=readcsv('labels.csv')
    photos=build_photos()
    # Exactly 62 / 104 records, approximately 60 percent, seeded and deterministic.
    ids=[r['record_id'] for r in labels];random.Random(SEED+1).shuffle(ids)
    shopee=set(ids[:round(.6*len(ids))])
    orders=[dict(order_id='SYN-'+r['record_id'],marketplace='shopee' if r['record_id'] in shopee else 'tiktok',
                 courier=r['courier'],resi=r['resi'],koli=int(r['koli']),listing_item=r['listing_item'],
                 created_at='2026-10-20T09:00:00+07:00',synthetic=True) for r in labels]
    dump(ROOT/'orders.json',orders)
    pairs=[]
    specs={r['record_id']:r for r in json.loads((ROOT/'tools/new-records.json').read_text())}
    for r in labels:
        if r['retake_of']:
            pairs.append(dict(pair_id=f'SYN-P{len(pairs)+1:02d}',bad_record=r['retake_of'],good_record=r['record_id'],
                              defect=specs[r['retake_of']]['condition'],resi=r['resi']))
    csvwrite(ROOT/'retake_pairs.csv',pairs)
    if not (ROOT/'split.csv').exists(): build_split(labels)
    build_claims(photos)
    from build_review import build
    build()
    print(f'Synthetic metadata: {len(labels)} records; {len(photos)} inspected photo proposals; 40 claims; {len(pairs)} pairs.')

if __name__=='__main__':main()

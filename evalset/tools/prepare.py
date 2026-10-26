"""Prepare synthetic assets without editing the read-only sources. No model APIs."""
import ast
import csv
import hashlib
import json
import random
import io
import re
import shutil
from pathlib import Path
from PIL import Image, ImageDraw, ImageOps

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT.parents[1]
SEED = 20261026

def dump(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')

def csvwrite(path, rows, fields=None):
    rows = list(rows)
    fields=list(fields or rows[0])
    if 'synthetic' not in fields:fields.append('synthetic')
    rows=[dict(r,synthetic='true') for r in rows]
    with path.open('w', newline='') as f:
        w = csv.DictWriter(f, fieldnames=fields, lineterminator='\n')
        w.writeheader()
        w.writerows(rows)

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def save_jpeg(im,path):
    # Compressed entropy bytes can accidentally resemble a contact token.
    # Try equivalent deterministic Huffman encodings from the same source pixels.
    phone=re.compile(rb'(?<![A-Za-z0-9])(?:08[0-9]{8,12}|\+62[0-9]{8,12})(?![0-9])')
    email=re.compile(rb'[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}')
    for options in ({},{'optimize':True},{'optimize':True,'progressive':True}):
        output=io.BytesIO()
        im.convert('RGB').save(output,format='JPEG',quality=95,subsampling=0,exif=b'',**options)
        raw=output.getvalue()
        if not phone.search(raw) and not email.search(raw):
            path.write_bytes(raw)
            return
    raise ValueError('No contact-token-free JPEG encoding for '+str(path))

def prepare():
    if (ROOT/'labels.csv').exists():
        raise RuntimeError('Initial preparation already exists. Use build_metadata.py to rebuild without resetting reviews or provenance.')
    snapshot = ROOT / 'tools/source-snapshot.json'
    if not snapshot.exists():
        paths = []
        for folder in ['jeejak-eval-set-synthetic', 'jeejak-eval-dump', 'jeejak-eval-set']:
            paths.extend(p for p in (WORK / folder).rglob('*') if p.is_file())
        dump(snapshot, {'synthetic':True,'files':{str(p.relative_to(WORK)): sha(p) for p in sorted(paths)}})
    original = list(csv.DictReader((WORK / 'jeejak-eval-set-synthetic/labels.csv').open()))
    jobs = json.loads((WORK / 'jeejak-eval-dump/generation-jobs.json').read_text())
    # Evaluate only the record-definition AST, before the generator's output code.
    source = (WORK / 'jeejak-eval-set/tools/gen_evalset.py').read_text()
    source = source[:source.index('# ---------------------------------------------------------------- checks')]
    tree = ast.parse(source)
    tree.body = [n for n in tree.body if not (isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'OUT' for t in n.targets))]
    ns = {'__file__': str(WORK / 'jeejak-eval-set/tools/gen_evalset.py')}
    exec(compile(tree, '<read-only-record-definitions>', 'exec'), ns)
    specs = {r['id']: r for r in ns['records']}
    for row in original:
        rid = row['record_id']
        dest = ROOT / 'records' / rid
        dest.mkdir(exist_ok=True)
        for src in sorted((WORK / 'jeejak-eval-set-synthetic' / rid).glob('*.jpg')):
            out = dest / src.name
            with Image.open(src) as im:
                if max(im.size) == 1280:
                    shutil.copyfile(src, out)
                else:
                    im.thumbnail((1280,1280), Image.Resampling.LANCZOS)
                    save_jpeg(im,out)
        row.update(group=specs[rid]['cat'].split(' (')[0].split(':')[0].replace(' ', '_'),
                   retake_of='', listing_item=specs[rid]['item'], label_source='proposed')
    rng = random.Random(SEED)
    couriers = ns['COURIERS'][:3]
    new = []
    def add(group, item='a white ceramic mug', condition='clean', portrait=False, pair=None, listing=None):
        rid = f'R{65+len(new):02d}'
        courier, gen = couriers[len(new) % 3]
        resi = gen(rng)
        if pair:
            courier, resi, item = pair['courier'], pair['resi'], pair['item']
        weak = condition in ('dark', 'blur')
        warning = condition in ('glare', 'too_far', 'cropped', 'open') or group == 'item_mismatch'
        row = dict(record_id=rid, resi=resi, koli='1', courier=courier,
                   image_usable={'dark':'dark','blur':'blurry'}.get(condition,'ok'),
                   label_visible='yes', label_resi_read='' if condition in ('dark','blur','glare','too_far','cropped') else resi,
                   package_sealed='no' if condition == 'open' else 'yes', item_visible='yes', koli_seen='1',
                   human_verdict='lemah' if weak else 'perlu_dicek' if warning else 'kuat',
                   group=group, retake_of=pair['record_id'] if pair else '', listing_item=listing or item,
                   label_source='proposed')
        spec = dict(**row, item=item, condition=condition, portrait=portrait, photos=3)
        original.append(row)
        new.append(spec)
        return spec
    for _ in range(4): add('glare', condition='glare')
    for c in ('too_far','too_far','cropped','cropped'): add('too_far_cropped',condition=c)
    # Two portrait records for each of the first three courier formats.
    for _ in range(6): add('portrait',portrait=True)
    for c in ('dark','dark','blur','blur','cropped','cropped','open','open'):
        bad = add('retake_pairs',condition=c)
        add('retake_pairs',condition='clean',pair=bad)
    for item in ('a lipstick set','a skincare bottle','earphones in a plain retail box','a phone charger',
                 'a glass candle jar in bubble wrap','a folded hijab'):
        add('icp_items',item=item)
    for item, listing in [('a blue ceramic mug','a red ceramic mug'),('a green folded hijab','a beige folded hijab'),
                          ('a black phone charger','a white phone charger'),('a red pencil case','a blue pencil case')]:
        add('item_mismatch',item=item,listing=listing)
    csvwrite(ROOT/'labels.csv',original)
    dump(ROOT/'tools/new-records.json',new)
    dump(ROOT/'tools/original-specs.json',{k:dict(v,synthetic=True) for k,v in specs.items()})
    prompts = []
    for r in new:
        label = f'Only these printed contents on the dummy waybill: "{r["courier"]}", "{r["resi"]}", one generic barcode, "PENERIMA: DUMMY", "ALAMAT: CONTOH". No other text, no names, phone numbers, street addresses, order ids, or record ids.'
        shape = 'Portrait phone framing, height greater than width.' if r['portrait'] else 'Landscape phone framing.'
        scenes = [f'{r["item"].capitalize()} INSIDE one open brown corrugated shipping box, all four flaps open, top-down view with contents visible. No tape or label yet.',
                  'The SAME box now closed and fully sealed with brown packing tape across the top seam and edges, whole box in frame, three-quarter view. Item inside hidden. White rectangular dummy waybill on top. '+label,
                  'Close-up of the SAME dummy waybill on the taped box, label fills half the frame, top seam tape visible. Item inside hidden. '+label]
        c = r['condition']
        for shot, scene in enumerate(scenes,1):
            defect = ''
            if c == 'dark': defect = 'VERY DIM light for this entire image. Strong underexposure. Shapes recognizable but label digits NOT readable. '
            if c == 'blur': defect = 'Strong real camera-motion blur across this entire image. Shapes recognizable, label digits NOT readable. '
            if shot > 1 and c == 'glare': defect = 'Broad white specular glare from glossy clear plastic over the label obscures the middle resi digits in EVERY label view; no fully readable resi. '
            if shot > 1 and c == 'too_far':
                scene = 'One fully taped box with a white dummy waybill on the table. '+label
                defect = 'Very distant wide view: parcel occupies less than one tenth of the frame width, lots of empty table. Label exists but digits are too small to read. No close-up. '
            if shot > 1 and c == 'cropped': defect = 'Camera framing cuts off the right half of the shipping label and parcel beyond the right image edge. The resi is truncated by the FRAME EDGE, never fully visible. '
            if shot > 1 and c == 'open':
                scene = f'The same box still OPEN with all flaps standing, NO tape anywhere. {r["item"]} visible inside. White dummy waybill stuck to SIDE of box. '+label
                if shot == 3: scene = 'Close-up of side waybill, open flaps clearly visible in background, NO tape. '+label
            prompt = ('Use case: photorealistic-natural. Create ONE synthetic smartphone photograph for a packing-evidence evaluation, '
                      f'{r["record_id"]} photo {shot:02d}. '+defect+scene+' Same plain light grey table and cream wall, ordinary everyday packing bench, realistic cardboard, no stylization. '
                      +('Natural even daylight. ' if c not in ('dark',) else '')+shape+
                      ' No people, no hands, no documents other than the specified dummy waybill, no personal information, no watermarks, no collage. '+
                      'Preserve the exact item, box, table and waybill identity across this sequence and its retake pair. This is synthetic, not a camera photo.')
            prompts.append(dict(record_id=r['record_id'],photo=f'{shot:02d}.jpg',prompt=prompt,synthetic=True,condition=c,
                                reference_record=r['retake_of'] or r['record_id']))
    dump(ROOT/'tools/generation-plan.json',prompts)
    dump(ROOT/'tools/source-prompts.json',jobs)
    print('Prepared 64 pre-existing records and 40 new record definitions; no image labels have been graded.')

def sheets(start=0, count=12):
    files = sorted((ROOT/'records').glob('*/*.jpg'),key=lambda p:(int(p.parent.name[1:]),int(p.stem)))[start:start+count]
    width, height = 500, 400
    canvas = Image.new('RGB',(width*3,height*((len(files)+2)//3)), 'white')
    draw = ImageDraw.Draw(canvas)
    for i,p in enumerate(files):
        with Image.open(p) as im:
            tile = ImageOps.contain(im,(width,height-28))
        x,y=(i%3)*width,(i//3)*height
        canvas.paste(tile,(x+(width-tile.width)//2,y+28))
        draw.text((x+10,y+7),f'{p.parent.name}/{p.name}   {Image.open(p).size}',fill='black')
    out=ROOT/'review'/f'inspection-{start:03d}.jpg'
    canvas.save(out,quality=93)
    print(out)

if __name__=='__main__':
    import sys
    if len(sys.argv)>1 and sys.argv[1]=='sheets': sheets(int(sys.argv[2]),int(sys.argv[3]) if len(sys.argv)>3 else 12)
    else: prepare()

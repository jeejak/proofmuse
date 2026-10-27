"""Derive record-level labels from human-reviewed photo labels. SYNTHETIC data. No API calls.

Samsul reviewed every photo (photos.csv, needs_review=no) but no record row.
The record columns ParcelProof used (image_usable, label_visible,
label_resi_read, package_sealed, item_visible, koli_seen, human_verdict)
are therefore recomputed here from his photo labels with fixed rules, so
no record value rests on a model proposal.

Rules, over all photos of a record:
- label_visible, package_sealed, item_visible: yes if any photo is yes,
  no if every photo is no, otherwise unsure.
- label_resi_read: the printed resi (the value already in the row, which
  for wrong-label records is the other resi) if any photo has a readable
  label, otherwise empty.
- koli_seen: the most shipping boxes visible in any one photo.
- image_usable: dark, blurry or obstructed only when every photo has
  that issue; a problem in some photos leaves the set usable.
- human_verdict and verdict_reasons: ParcelProof's Decide rules
  (evidencecheck/verdict.go) applied to the values above, plus one
  ProofMuse reason: item_mismatch, set by construction for the
  item_mismatch group (the item shown differs from the order listing; no
  photo field captures it), which is a perlu_dicek reason.

Rows whose record_fields_source is "human" (an explicit record review)
are left alone.
"""
import csv
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FIELDS = ['image_usable', 'label_visible', 'label_resi_read', 'package_sealed',
          'item_visible', 'koli_seen', 'human_verdict']
WEAK = {'image_dark', 'image_blurry', 'image_obstructed', 'label_not_visible',
        'label_mismatch', 'item_not_visible'}
SOURCE = 'derived_from_human_photos'


def tri(values):
    if 'yes' in values:
        return 'yes'
    if values and all(v == 'no' for v in values):
        return 'no'
    return 'unsure'


def usable(photos):
    issues = [set(filter(None, p['image_quality_issues'].split('|'))) for p in photos]
    for issue, value in (('too_dark', 'dark'), ('blur', 'blurry'), ('obstructed', 'obstructed')):
        if issues and all(issue in s for s in issues):
            return value
    return 'ok'


def norm(s):
    return re.sub('[^A-Z0-9]', '', s.upper())


def decide(rec, resi, koli, group):
    reasons = []
    usable_reason = {'dark': 'image_dark', 'blurry': 'image_blurry', 'obstructed': 'image_obstructed'}
    if rec['image_usable'] in usable_reason:
        reasons.append(usable_reason[rec['image_usable']])
    read = norm(rec['label_resi_read'])
    if read:
        if read != norm(resi):
            reasons.append('label_mismatch')
    elif rec['label_visible'] == 'no':
        reasons.append('label_not_visible')
    elif rec['label_visible'] == 'yes':
        reasons.append('label_unreadable')
    if rec['item_visible'] == 'no':
        reasons.append('item_not_visible')
    if rec['package_sealed'] == 'no':
        reasons.append('not_sealed')
    if rec['koli_seen'] and int(rec['koli_seen']) != int(koli):
        reasons.append('koli_mismatch')
    if group == 'item_mismatch':
        reasons.append('item_mismatch')
    verdict = 'kuat'
    for r in reasons:
        if r in WEAK:
            verdict = 'lemah'
            break
        verdict = 'perlu_dicek'
    return verdict, reasons


def derive(labels, photos):
    by = {}
    for p in photos:
        by.setdefault(p['record_id'], []).append(p)
    out = []
    for r in labels:
        r = dict(r)
        ps = by[r['record_id']]
        if r.get('record_fields_source') == 'human':
            out.append(r)
            continue
        if any(p['needs_review'] != 'no' for p in ps):
            raise SystemExit(r['record_id'] + ': photos not all human-reviewed; cannot derive')
        readable = any(p['label_readable'] == 'yes' for p in ps)
        if readable and not r['label_resi_read']:
            raise SystemExit(r['record_id'] + ': readable label photo but no printed resi in the row')
        d = {
            'image_usable': usable(ps),
            'label_visible': tri([p['label_in_photo'] for p in ps]),
            'label_resi_read': r['label_resi_read'] if readable else '',
            'package_sealed': tri([p['sealed_box_in_photo'] for p in ps]),
            'item_visible': tri([p['item_in_photo'] for p in ps]),
            'koli_seen': str(max(int(p['boxes_in_photo']) for p in ps)),
        }
        d['human_verdict'], reasons = decide(d, r['resi'], r['koli'], r['group'])
        r.update(d)
        r['verdict_reasons'] = '|'.join(reasons)
        r['record_fields_source'] = SOURCE
        out.append(r)
    return out


def main():
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT
    with (root / 'labels.csv').open() as f:
        labels = list(csv.DictReader(f))
    with (root / 'photos.csv').open() as f:
        photos = list(csv.DictReader(f))
    rows = derive(labels, photos)
    fields = list(labels[0].keys())
    if 'verdict_reasons' not in fields:
        fields.insert(fields.index('human_verdict') + 1, 'verdict_reasons')
    changed = sum(1 for a, b in zip(labels, rows) if any(a[k] != b[k] for k in FIELDS))
    with (root / 'labels.csv').open('w', newline='') as f:
        w = csv.DictWriter(f, fieldnames=fields, lineterminator='\n')
        w.writeheader()
        for r in rows:
            r.setdefault('verdict_reasons', '')
            w.writerow({k: r.get(k, '') for k in fields})
    print(f'Derived {len(rows)} records from reviewed photo labels; {changed} changed.')


if __name__ == '__main__':
    main()

"""Reconcile SYNTHETIC new record proposals with pixel-inspected photo observations."""
import csv
from collections import defaultdict
from prepare import ROOT,csvwrite

def main():
    labels=list(csv.DictReader((ROOT/'labels.csv').open()))
    photos=list(csv.DictReader((ROOT/'photos.csv').open()))
    by=defaultdict(list)
    for p in photos:by[p['record_id']].append(p)
    for r in labels:
        r.setdefault('record_fields_source','proposed')
        r.setdefault('record_reviewed_by','');r.setdefault('record_reviewed_at','')
        if int(r['record_id'][1:])<65 or r['record_fields_source']=='human':continue
        ps=by[r['record_id']]
        # Full readable numbers were visually compared with the saved resi.
        # Never infer a number from a crop or unreadable proposal.
        r['label_resi_read']=r['resi'] if any(p['label_readable']=='yes' for p in ps) else ''
        r['label_visible']='yes' if any(p['label_in_photo']=='yes' for p in ps) else 'no'
        r['package_sealed']='yes' if any(p['sealed_box_in_photo']=='yes' for p in ps) else 'no'
        r['item_visible']='yes' if any(p['item_in_photo']=='yes' for p in ps) else 'no'
        r['koli_seen']=str(max(int(p['boxes_in_photo']) for p in ps))
        # Whole-record unusability is reserved for sequences consistently dark or blurred.
        issues=[set(p['image_quality_issues'].split('|')) for p in ps]
        r['image_usable']='dark' if all('too_dark' in q for q in issues) else 'blurry' if all('blur' in q for q in issues) else 'ok'
        if r['image_usable']!='ok':r['human_verdict']='lemah'
        elif not r['label_resi_read'] or r['package_sealed']!='yes' or r['group']=='item_mismatch':r['human_verdict']='perlu_dicek'
        else:r['human_verdict']='kuat'
    csvwrite(ROOT/'labels.csv',labels)
    print('Reconciled synthetic new record proposals; original label columns and human record reviews preserved.')

if __name__=='__main__':main()

"""Offline tests for explicit review merges of SYNTHETIC proposals."""
import csv
import shutil
import tempfile
from pathlib import Path
from apply_review import merge
from prepare import ROOT,csvwrite

def read(p):return list(csv.DictReader(p.open()))

def main():
    with tempfile.TemporaryDirectory() as folder:
        root=Path(folder)
        for name in ('photos.csv','labels.csv'):shutil.copyfile(ROOT/name,root/name)
        # The live set is already reviewed; reset the copy to its pre-review
        # state so this test exercises a first review on any snapshot.
        fields=list(read(root/'photos.csv')[0].keys())
        reset=[dict(p,needs_review='yes',reviewed_by='',reviewed_at='') for p in read(root/'photos.csv')]
        csvwrite(root/'photos.csv',reset,fields)
        lfields=list(read(root/'labels.csv')[0].keys())
        csvwrite(root/'labels.csv',[dict(r,label_source='proposed',record_fields_source='proposed',record_reviewed_by='',record_reviewed_at='') for r in read(root/'labels.csv')],lfields)
        original=read(root/'photos.csv');rid=original[0]['record_id']
        rows=[p for p in original if p['record_id']==rid]
        decisions=[]
        for p in rows:
            decisions.append(dict(p,photo_review='yes',reviewed_by='Test Reviewer',reviewed_at='2026-10-26T12:00:00+07:00',record_review='no'))
        path=root/'decisions.csv';csvwrite(path,decisions[:1]);merge(root,path)
        now=read(root/'photos.csv')
        assert sum(p['needs_review']=='no' for p in now)==1
        assert [p for p in now if p['record_id']!=rid]==[p for p in original if p['record_id']!=rid]
        assert next(r for r in read(root/'labels.csv') if r['record_id']==rid)['label_source']=='proposed'
        csvwrite(path,decisions);merge(root,path)
        record=next(r for r in read(root/'labels.csv') if r['record_id']==rid)
        assert record['label_source']=='human' and record['record_fields_source']=='proposed'
        before=[(root/n).read_bytes() for n in ('photos.csv','labels.csv')]
        merge(root,path)
        assert before==[(root/n).read_bytes() for n in ('photos.csv','labels.csv')]
        record_decision=dict(decisions[0],record_review='yes',record_reviewed_by='Test Reviewer',record_reviewed_at='2026-10-26T12:00:00+07:00')
        from apply_review import RECORD_LABELS
        record_decision.update({'record_'+f:record[f] for f in RECORD_LABELS})
        csvwrite(path,[record_decision]);merge(root,path)
        assert next(r for r in read(root/'labels.csv') if r['record_id']==rid)['record_fields_source']=='human'
        before=[(root/n).read_bytes() for n in ('photos.csv','labels.csv')]
        bad=dict(decisions[0],label_box='900 0 100 100')
        csvwrite(path,[bad])
        try:merge(root,path)
        except ValueError:pass
        else:raise AssertionError('Invalid decision accepted')
        assert before==[(root/n).read_bytes() for n in ('photos.csv','labels.csv')]
        bad=dict(decisions[0],record_id='R999');csvwrite(path,[bad])
        try:merge(root,path)
        except ValueError:pass
        else:raise AssertionError('Unknown photo accepted')
        assert before==[(root/n).read_bytes() for n in ('photos.csv','labels.csv')]
    print('PASS: synthetic review partial merge, untouched rows, full-record gate, idempotence and rejection without writes.')

if __name__=='__main__':main()

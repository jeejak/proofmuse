"""Build the offline SYNTHETIC proposal review page. No server or network requests."""
import json
from prepare import ROOT
from build_metadata import readcsv

def build():
    split={r['record_id']:r['split'] for r in readcsv('split.csv')}
    labels={r['record_id']:r for r in readcsv('labels.csv')}
    photos=readcsv('photos.csv')
    photos.sort(key=lambda p:(0 if split[p['record_id']]=='test' else 1,int(p['record_id'][1:]),p['photo']))
    data=[dict(p,split=split[p['record_id']],record=labels[p['record_id']]) for p in photos]
    template=(ROOT/'tools/review-template.html').read_text()
    (ROOT/'review/index.html').write_text(template.replace('/*DATA*/[]',json.dumps(data,ensure_ascii=False).replace('</','<\\/')))

if __name__=='__main__':build()

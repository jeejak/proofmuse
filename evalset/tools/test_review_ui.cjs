/* Offline unit tests for SYNTHETIC review-page logic using an isolated fake DOM. */
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict');
class Element{
 constructor(){this.value='';this.checked=false;this.hidden=false;this.style={};this.children=[];this.textContent='';this.attributes={};}
 append(e){this.children.push(e);if(e.id)elements[e.id]=e;}
 addEventListener(){}
 setAttribute(k,v){this.attributes[k]=String(v);}
 click(){if(this.onclick)this.onclick();}
}
const elements={};
const html=fs.readFileSync(path.join(__dirname,'../review/index.html'),'utf8');
for(const id of html.matchAll(/id="([^"]+)"/g))elements[id[1]]=new Element();
elements.blind.checked=true;
elements.reviewer.value='Test Reviewer';
let savedBlob=null;
const ctx=vm.createContext({document:{getElementById:id=>elements[id],createElement:()=>new Element()},
 localStorage:{getItem:()=>null,setItem(){}},window:{open(){}},Date,Blob,
 URL:{createObjectURL:b=>{savedBlob=b;return 'blob:synthetic-test'},revokeObjectURL(){}},setTimeout:f=>f()});
vm.runInContext(html.match(/<script>([\s\S]*)<\/script>/)[1],ctx);
assert.equal(elements.blind.checked,true);
assert.equal(elements.comparison.hidden,true);
assert.equal(elements.accept.hidden,true);
assert.equal(elements.proposedbox.style.display,'none');
for(const f of ['label_in_photo','label_readable','image_quality_issues','boxes_in_photo','sealed_box_in_photo','item_in_photo','label_box'])assert.equal(elements[f].value,'');
assert.match(elements.title.textContent,/test.*SYNTHETIC/);
elements.save.click();assert.match(elements.message.textContent,/Enter/);
Object.assign(elements.label_in_photo,{value:'no'});elements.label_readable.value='no';elements.boxes_in_photo.value='1';elements.sealed_box_in_photo.value='no';elements.item_in_photo.value='yes';
elements.save.click();assert.equal(elements.comparison.hidden,false);assert.equal(elements.accept.hidden,false);assert.match(elements.progress.textContent,/1 \/ /);
elements.export.click();assert.ok(savedBlob);
savedBlob.text().then(csv=>{assert.match(csv,/review_mode/);assert.match(csv,/"blind"/);assert.match(csv,/Test Reviewer/);assert.match(csv,/\+07:00/);assert.match(csv,/synthetic/);elements.next.click();assert.equal(elements.comparison.hidden,true);assert.equal(elements.label_in_photo.value,'');elements.blind.checked=false;elements.blind.onchange();assert.equal(elements.comparison.hidden,false);console.log('PASS: synthetic review blind defaults, independent input gate, compare, next-photo reset and CSV export. Fake DOM only; visual browser rendering not tested.');}).catch(e=>{console.error(e);process.exitCode=1});

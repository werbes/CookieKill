const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),read=file=>fs.readFileSync(path.join(root,file),'utf8'),load=file=>import('data:text/javascript;base64,'+Buffer.from(read(file)).toString('base64'));
const catalog=[...read('internal/game/achievements.go').matchAll(/\{"([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)"\}/g)].map(([,id,name,description,hat,icon,color])=>({id,name,description,hat,icon,color}));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/achievement-panels.js'),load('web/achievement-scenery.js')]);
test('catalog-driven hats and gallery distinguish earned, worn and hung, with local owner action guards',async()=>{
  const [,{createAchievementPanels}]=await modules,buttons=[],p={id:'me',bakery:true,bakeryPlot:'plot',x:0,y:0,z:0,hats:{chef:true},achievements:{chef:true,ocean:true,builder:true,merchant:true},hungAchievements:['chef','ocean','builder','merchant','chef','racer']};
  const state={connected:true,snapshot:{me:p,achievementCatalog:catalog,hatCatalog:catalog.map(a=>({id:a.hat,achievement:a.id,name:a.name,icon:a.icon})),nodes:[{id:'plot',kind:'bakery_plot',owner:'me',x:0,z:0}]}};
  const api=createAchievementPanels({state,escapeHTML:s=>s.replaceAll('&','&amp;').replaceAll('<','&lt;'),button:(name,action,data,disabled)=>{buttons.push({name,action,data,disabled});return '<button>'+name+'</button>';}});
  assert.match(api.hats({hat:'chef'}),/✓ First batch/);assert.equal(buttons.length,16);assert.equal(buttons.find(b=>b.data.hat==='racer').disabled,true);assert.equal(api.preview('chef'),'♧');
  buttons.length=0;assert.match(api.gallery(),/1 \+ 0.10 × 4 = 1.4×/);assert.equal(buttons.find(b=>b.data.item==='chef').action,'unhang_achievement');assert.equal(buttons.find(b=>b.data.item==='chef').disabled,false);assert.equal(buttons.find(b=>b.data.item==='racer').disabled,true);
  for(const change of [{x:11},{island:'home'},{inCave:true},{y:3}]){const before={...p};Object.assign(p,change);buttons.length=0;api.gallery();assert.ok(buttons.every(b=>b.disabled));Object.assign(p,before);delete p.island;delete p.inCave;}
  state.snapshot.nodes[0].owner='other';buttons.length=0;api.gallery();assert.ok(buttons.every(b=>b.disabled));
});
test('all fifteen earned hats have distinct supported rigs and no fallback for unknown IDs',async()=>{
  const [THREE,,{buildAchievementHat}]=await modules,parent=new THREE.Group(),deps=primitives(THREE),signatures=new Set();
  for(const a of catalog){const h=buildAchievementHat({...deps,parent},a.hat);assert.equal(h.name,'achievement-hat-'+a.hat);assert.ok(h.children.length>0);assert.equal(h.position.y,1.9);const bounds=new THREE.Box3().setFromObject(h);assert.ok(bounds.min.y>1.65&&bounds.max.y<2.7,a.hat+JSON.stringify({min:bounds.min.y,max:bounds.max.y}));signatures.add(JSON.stringify(h.children.map(c=>[c.geometry.type,c.geometry.parameters,c.position.toArray(),c.material.color.getHex()])));}
  assert.equal(catalog.length,15);assert.equal(signatures.size,15);assert.equal(buildAchievementHat({...deps,parent},'fake'),null);assert.equal(parent.children.length,15);
});
function primitives(THREE){const primitive=(geometry,color,x,y,z,parent)=>{const m=new THREE.Mesh(geometry,new THREE.MeshBasicMaterial({color}));m.position.set(x,y,z);parent.add(m);return m;};return {THREE,box:(w,h,d,c,x,y,z,p)=>primitive(new THREE.BoxGeometry(w,h,d),c,x,y,z,p),sphere:(r,c,x,y,z,p)=>primitive(new THREE.SphereGeometry(r,8,6),c,x,y,z,p),cylinder:(a,b,h,c,x,y,z,p,n=8)=>primitive(new THREE.CylinderGeometry(a,b,h,n),c,x,y,z,p)};}
test('wall gallery dedupes, ignores unknown frames, updates ownership, and stays inside cafe wall bounds',async()=>{
  const [THREE,,{createAchievementScenery}]=await modules,parent=new THREE.Group(),removed=[];
  const api=createAchievementScenery({...primitives(THREE),parent,flatLabel:text=>{const g=new THREE.Group();g.userData.text=text;return g;},disposeObject:o=>removed.push(o)});
  const snapshot={me:{},achievementCatalog:catalog,nodes:[{id:'plot',kind:'bakery_plot',owner:'one',x:30,z:40,hungAchievements:catalog.map(a=>a.id).concat(['chef','fake'])}]};api.sync(snapshot);
  const gallery=api.root.children[0];assert.equal(gallery.children.length,15);assert.equal(gallery.position.x,30);for(const frame of gallery.children){assert.equal(frame.position.x,9.73);assert.ok(frame.position.z>-.8&&frame.position.z<7.4&&frame.position.y<5);}
  api.sync(snapshot);assert.equal(api.root.children[0],gallery,'unchanged snapshot must reuse render resources');snapshot.nodes[0].hungAchievements=['chef'];api.sync(snapshot);assert.equal(api.root.children[0].children.length,1);assert.equal(removed.length,1);
  snapshot.me.island='home';api.sync(snapshot);assert.equal(api.root.visible,false);snapshot.nodes=[];api.sync(snapshot);assert.equal(api.root.children.length,0);api.dispose();assert.equal(parent.children.length,0);
});

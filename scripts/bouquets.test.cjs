const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'..',file),'utf8')).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/bouquet-view.js'),load('web/town-panels.js')]);
const bouquets=[{id:'flower_bouquet',name:'A little sunshine',price:20},{id:'bouquet_daisies',name:'Sunny daisies',price:15},{id:'bouquet_tulips',name:'Rosy tulips',price:30},{id:'bouquet_lavender',name:'Lavender wishes',price:25}];
test('four bouquets remain distinct in shop prices, inventory labels and exact gifted item payloads',async()=>{
  const [,,{createTownPanels}]=await modules,buttons=[],state={connected:true,snapshot:{town:{bouquets,bouquet:bouquets[0]},me:{id:'me',x:0,z:0,coins:25,inventory:{bouquet_tulips:1}},nodes:[{id:'flower_shop',kind:'flower_shop',x:0,z:0}],players:[{id:'friend',name:'Mia',x:1,z:0}]}};
  const panel=createTownPanels({state,escapeHTML:String,renderModal(){},shopRow:(icon,name,description,b)=>name+description+b,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  panel.shop('flower_shop');assert.equal(buttons.length,4);assert.deepEqual(buttons.map(b=>b.data.item),bouquets.map(b=>b.id));assert.equal(buttons[2].disabled,true);assert.ok(buttons.filter((_,i)=>i!==2).every(b=>!b.disabled));
  for(const b of bouquets){assert.equal(panel.itemName(b.id),b.name);assert.equal(panel.isTownItem(b.id),true);assert.equal(panel.itemIcon(b.id),'💐');}
  buttons.length=0;panel.giftControls('bouquet_tulips');assert.deepEqual(buttons[0].data,{target:'friend',item:'bouquet_tulips'});assert.equal(buttons[0].disabled,false);assert.equal(panel.giftControls('fake'),'');
  delete state.snapshot.town.bouquets;buttons.length=0;panel.shop('flower_shop');assert.equal(buttons.length,1);assert.equal(buttons[0].data.item,'flower_bouquet','legacy snapshots retain the original bouquet');
});
test('held bouquet meshes have distinct flower shapes and wrapping, within a first person hand footprint',async()=>{
  const [THREE,{createBouquetView}]=await modules;
  const primitive=(geometry,color,x,y,z,p)=>{const m=new THREE.Mesh(geometry,new THREE.MeshBasicMaterial({color}));m.position.set(x,y,z);p.add(m);return m;};
  const deps={THREE,box:(w,h,d,c,x,y,z,p)=>primitive(new THREE.BoxGeometry(w,h,d),c,x,y,z,p),sphere:(r,c,x,y,z,p)=>primitive(new THREE.IcosahedronGeometry(r,1),c,x,y,z,p),cylinder:(a,b,h,c,x,y,z,p,n=8)=>primitive(new THREE.CylinderGeometry(a,b,h,n),c,x,y,z,p)};
  const models=bouquets.map(b=>createBouquetView(deps,b.id));assert.equal(new Set(models.map(m=>m.userData.bouquetKind)).size,4);assert.equal(new Set(models.map(m=>m.children[0].material.color.getHex())).size,4);
  for(let i=0;i<models.length;i++){const m=models[i],bounds=new THREE.Box3().setFromObject(m),size=bounds.getSize(new THREE.Vector3());assert.equal(m.name,'held-bouquet-'+bouquets[i].id);assert.ok(size.x<.7&&size.y<1&&size.z<.7);assert.ok(m.children.filter(c=>c.name.startsWith('bouquet-flower-')).length>=5);}
  assert.equal(createBouquetView(deps,'cake'),null);
});

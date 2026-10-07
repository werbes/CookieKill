const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,file),'utf8')).toString('base64'));
const modulePromise=load('web/bakery-panels.js');
const escapeHTML=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function fixture(createBakeryPanels){
 const state={snapshot:{me:{id:'owner',coins:500,inventory:{sugar:10,dough:7,berry:2},bakeryDisplay:{sugar:5},bakeryPantry:{berry:3},bakeryStaff:{},bakeryChefRecipe:'sugar',discovered:{berry_cookie:true},bakeryBusiness:{displayCapacity:100,pantryCapacity:1000,customers:[]}},recipes:[{id:'sugar',name:'Sugar cookie',cost:{dough:1}},{id:'berry_cookie',name:'Berry burst',cost:{dough:1,berry:2}},{id:'cake',name:'Cake',cost:{dough:3,berry:2,nut:2}}]}};
 const button=(label,kind,data={},disabled=false)=>`<button data-action="${kind}" data-payload='${JSON.stringify(data)}'${disabled?' disabled':''}>${escapeHTML(label)}</button>`;
 const panel=createBakeryPanels({state,button,escapeHTML,insideOwnBakery:()=>!!state.snapshot.me.insideBakery,pretty:value=>String(value).replaceAll('_',' '),itemName:id=>id,shopRow:(_icon,title,description,buttons)=>`<article><h3>${escapeHTML(title)}</h3><p>${escapeHTML(description)}</p>${buttons}</article>`,renderModal:()=>{}});
 return {state,p:state.snapshot.me,panel};
}
const buttons=(html,action)=>Array.from(html.matchAll(/<button data-action="([^"]+)" data-payload='([^']+)'([^>]*)>(.*?)<\/button>/g)).filter(match=>match[1]===action).map(match=>({data:JSON.parse(match[2]),disabled:match[3].includes('disabled'),label:match[4]}));
function assertCatalogStructure(html){
 const stack=[];let catalogs=0;
 for(const match of html.matchAll(/<\/?div\b[^>]*>/g)){
  if(match[0].startsWith('</')){assert.ok(stack.length,'closing div has an opening div');stack.pop();continue;}
  const classes=match[0].match(/class="([^"]*)"/)?.[1].split(' ')||[];
  if(classes.includes('build-catalog')){catalogs++;assert.equal(stack.length,0,'catalog must be outside all notices');}
  if(classes.includes('notice'))assert.equal(stack.length,0,'notices must be siblings rather than accidentally nested');
  stack.push(classes);
 }
 assert.equal(stack.length,0,'all divs are closed');assert.equal(catalogs,1);
}

test('display controls transfer real stock and offer only waiting, fulfillable orders',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels),node={id:'plot_display'};
 let html=panel.display(node);assert.doesNotMatch(html,/data-action="trade"|Sell one/);let stock=buttons(html,'bakery_stock').find(b=>b.data.item==='sugar');assert.deepEqual(stock.data,{target:'plot_display',item:'sugar',amount:5});assert.equal(stock.disabled,false);
 p.bakeryDisplay.sugar=99;html=panel.display(node);assert.equal(buttons(html,'bakery_stock').find(b=>b.data.item==='sugar').data.amount,1);p.bakeryDisplay.sugar=100;assert.equal(buttons(panel.display(node),'bakery_stock').find(b=>b.data.item==='sugar').disabled,true);
 p.bakeryDisplay.sugar=5;p.bakeryBusiness.customers=[{id:'one',item:'sugar',amount:5,phase:'waiting',price:17},{id:'two',item:'sugar',amount:1,phase:'arriving',price:3},{id:'gone',item:'sugar',amount:0,phase:'leaving',price:0}];
 let serve=buttons(panel.display(node),'bakery_serve');assert.equal(serve.length,1);assert.deepEqual(serve[0].data,{target:'one'});assert.match(serve[0].label,/17 coins/);assert.equal(serve[0].disabled,false);
 p.bakeryDisplay.sugar=4;html=panel.display(node);assert.equal(buttons(html,'bakery_serve')[0].disabled,true);assert.match(html,/Needs 1 more on the display/);
 panel.handle('bakery-transfer-size',{amount:20});p.inventory.sugar=7;assert.equal(buttons(panel.display(node),'bakery_stock').find(b=>b.data.item==='sugar').data.amount,7);
});

test('bakery furnishing catalog separates core limits, coin purchases, painted packed items and build permission',async()=>{
 const {createBakeryPanels}=await modulePromise,{state,p,panel}=fixture(createBakeryPanels);p.bakeryPlot='plot';p.insideBakery=true;p.bakeryBuildMode=true;p.bakeryFurniture=[{id:'oven1',kind:'oven',color:'teal'},{id:'chair1',kind:'chair'}];p.inventory['bakery_chair:blue']=1;p.inventory.paint_red=1;p.editingBakeryFurnitureId='chair1';state.snapshot.bakeryFurnitureCatalog=[{id:'oven',name:'Bakery oven',width:2.2,depth:1.4,room:'kitchen',cost:120,core:true},{id:'chair',name:'Cafe chair',width:.9,depth:.9,room:'cafe',cost:12}];
 let html=panel.building(),choices=buttons(html,'bakery-place-choice');assert.equal(choices[0].disabled,true);assert.equal(choices[1].disabled,false);assert.deepEqual(choices[2].data,{item:'chair',packed:'bakery_chair:blue'});assert.equal(choices[2].disabled,false);assert.match(html,/Packed ovens, mixers, or displays pause/);assert.equal(buttons(html,'bakery_paint').find(b=>b.data.item==='red').disabled,false);
 assertCatalogStructure(html);p.editingBakeryFurnitureId='';assertCatalogStructure(panel.building());
 p.coins=0;choices=buttons(panel.building(),'bakery-place-choice');assert.equal(choices[1].disabled,true);assert.equal(choices[2].disabled,false);
 p.insideBakery=false;assert.ok(buttons(panel.building(),'bakery-place-choice').every(b=>b.disabled));p.insideBakery=true;p.bakeryBuildMode=false;assert.ok(buttons(panel.building(),'bakery-place-choice').every(b=>b.disabled));
});

test('pantry capacity, persistent hires and discovered chef recipes have separate controls',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels);
 const deposit=buttons(panel.pantry({id:'plot_mixer'}),'bakery_pantry_deposit').find(b=>b.data.item==='dough');assert.deepEqual(deposit.data,{target:'plot_mixer',item:'dough',amount:5});assert.equal(deposit.disabled,false);
 p.bakeryPantry={berry:1000};assert.ok(buttons(panel.pantry({id:'plot_mixer'}),'bakery_pantry_deposit').every(b=>b.disabled));
 let hires=buttons(panel.staff({id:'plot'}),'bakery_hire');assert.deepEqual(hires.map(b=>b.data),[{target:'plot',item:'chef'},{target:'plot',item:'cashier'}]);assert.ok(hires.every(b=>!b.disabled));
 p.coins=199;p.bakeryStaff.chef=true;hires=buttons(panel.staff({id:'plot'}),'bakery_hire');assert.ok(hires.every(b=>b.disabled));
 const choices=buttons(panel.chef({id:'plot_oven'}),'bakery_chef_recipe');assert.deepEqual(choices.map(b=>b.data.item),['sugar','berry_cookie']);assert.equal(choices[0].disabled,true);assert.deepEqual(choices[1].data,{target:'plot_oven',item:'berry_cookie'});
 assert.match(panel.staff({id:'plot'}),/pause when you disconnect/);assert.match(panel.pantry({id:'plot_mixer'}),/personal inventory stay there/);
});

const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,file),'utf8')).toString('base64'));
const modulePromise=load('web/bakery-panels.js');
const escapeHTML=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function fixture(createBakeryPanels,extra={}){
 const state={snapshot:{me:{id:'owner',coins:500,inventory:{sugar:10,dough:7,berry:2},bakeryDisplay:{sugar:5},bakeryPantry:{berry:3},bakeryStaff:{},bakeryChefRecipe:'sugar',discovered:{berry_cookie:true},bakeryBusiness:{displayCapacity:300,pantryCapacity:1000,customers:[]}},recipes:[{id:'sugar',name:'Sugar cookie',cost:{dough:1}},{id:'berry_cookie',name:'Berry burst',cost:{dough:1,berry:2}},{id:'cake',name:'Cake',cost:{dough:3,berry:2,nut:2}}]}};
 const button=(label,kind,data={},disabled=false)=>`<button data-action="${kind}" data-payload='${JSON.stringify(data)}'${disabled?' disabled':''}>${escapeHTML(label)}</button>`;
 const panel=createBakeryPanels({state,button,escapeHTML,insideOwnBakery:()=>!!state.snapshot.me.insideBakery,pretty:value=>String(value).replaceAll('_',' '),itemName:id=>id,shopRow:(_icon,title,description,buttons)=>`<article><h3>${escapeHTML(title)}</h3><p>${escapeHTML(description)}</p>${buttons}</article>`,renderModal:()=>{},...extra});
 return {state,p:state.snapshot.me,panel};
}
const buttons=(html,action)=>Array.from(html.matchAll(/<button data-action="([^"]+)" data-payload='([^']+)'([^>]*)>(.*?)<\/button>/g)).filter(match=>match[1]===action).map(match=>({data:JSON.parse(match[2]),disabled:match[3].includes('disabled'),label:match[4]}));

test('bakery building has persistent distinct furniture and achievement painting categories',async()=>{
 const {createBakeryPanels}=await modulePromise;let renders=0,paintings=0;const sent=[];
 const {state,p,panel}=fixture(createBakeryPanels,{renderModal:()=>renders++,action:(...a)=>sent.push(a),renderAchievementPaintings:()=>{paintings++;return '<section>PAINTINGS ONLY</section>';}});
 Object.assign(p,{bakeryPlot:'plot',insideBakery:true,bakeryBuildMode:false});state.snapshot.bakeryFurnitureCatalog=[{id:'chair',name:'Cafe chair',width:.9,depth:.9,room:'cafe',cost:12}];
 let html=panel.building();assert.equal(paintings,0);assert.match(html,/BAKERY FURNISHINGS/);assert.doesNotMatch(html,/PAINTINGS ONLY/);assert.deepEqual(buttons(html,'bakery-build-category').map(b=>[b.label,b.data.category,b.disabled]),[['Furniture','furniture',false],['Achievement paintings','achievements',false]]);
 assert.equal(panel.handle('bakery-build-category',{category:'achievements'}),true);html=panel.building();assert.equal(renders,1);assert.match(html,/PAINTINGS ONLY/);assert.doesNotMatch(html,/BAKERY FURNISHINGS|bakery-place-choice|Enter bakery build mode/);assert.equal(p.bakeryBuildMode,false,'paintings are accessible in explore mode');
 p.coins=999;assert.match(panel.building(),/PAINTINGS ONLY/,'snapshot refresh retains selected category');assert.equal(panel.handle('bakery-build-category',{category:'untrusted'}),true);assert.equal(renders,1);assert.match(panel.building(),/PAINTINGS ONLY/);assert.deepEqual(sent,[],'category changes are local, not server actions');
 panel.handle('bakery-build-category',{category:'furniture'});html=panel.building();assert.match(html,/BAKERY FURNISHINGS/);assert.doesNotMatch(html,/PAINTINGS ONLY/);assert.equal(renders,2);
});
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
 p.bakeryDisplay.sugar=299;html=panel.display(node);assert.equal(buttons(html,'bakery_stock').find(b=>b.data.item==='sugar').data.amount,1);p.bakeryDisplay.sugar=300;assert.equal(buttons(panel.display(node),'bakery_stock').find(b=>b.data.item==='sugar').disabled,true);
 p.bakeryDisplay.sugar=5;p.bakeryBusiness.customers=[{id:'one',item:'sugar',amount:5,phase:'waiting',price:17},{id:'two',item:'sugar',amount:1,phase:'arriving',price:3},{id:'gone',item:'sugar',amount:0,phase:'leaving',price:0}];
 let serve=buttons(panel.display(node),'bakery_serve');assert.equal(serve.length,1);assert.deepEqual(serve[0].data,{target:'one'});assert.match(serve[0].label,/17 coins/);assert.equal(serve[0].disabled,false);
 p.bakeryDisplay.sugar=4;html=panel.display(node);assert.equal(buttons(html,'bakery_serve')[0].disabled,true);assert.match(html,/Needs 1 more on the display/);
 panel.handle('bakery-transfer-size',{amount:20});p.inventory.sugar=7;assert.equal(buttons(panel.display(node),'bakery_stock').find(b=>b.data.item==='sugar').data.amount,7);
});

test('bakery furnishing catalog separates core limits, coin purchases, painted packed items and build permission',async()=>{
 const {createBakeryPanels}=await modulePromise,{state,p,panel}=fixture(createBakeryPanels);p.bakeryPlot='plot';p.insideBakery=true;p.bakeryBuildMode=true;p.bakeryFurniture=[{id:'oven1',kind:'oven',color:'teal'},{id:'chair1',kind:'chair'}];p.inventory['bakery_chair:blue']=1;p.inventory.paint_red=1;p.editingBakeryFurnitureId='chair1';state.snapshot.bakeryFurnitureCatalog=[{id:'oven',name:'Bakery oven',width:2.2,depth:1.4,room:'kitchen',cost:120,core:true},{id:'chair',name:'Cafe chair',width:.9,depth:.9,room:'cafe',cost:12}];
 let html=panel.building(),choices=buttons(html,'bakery-place-choice');assert.equal(choices[0].disabled,true);assert.equal(choices[1].disabled,false);assert.deepEqual(choices[2].data,{item:'chair',packed:'bakery_chair:blue'});assert.equal(choices[2].disabled,false);assert.match(html,/active or queued station batches before packing/);assert.equal(buttons(html,'bakery_paint').find(b=>b.data.item==='red').disabled,false);
 assertCatalogStructure(html);p.editingBakeryFurnitureId='';assertCatalogStructure(panel.building());
 p.coins=0;choices=buttons(panel.building(),'bakery-place-choice');assert.equal(choices[1].disabled,true);assert.equal(choices[2].disabled,false);
 p.insideBakery=false;assert.ok(buttons(panel.building(),'bakery-place-choice').every(b=>b.disabled));p.insideBakery=true;p.bakeryBuildMode=false;assert.ok(buttons(panel.building(),'bakery-place-choice').every(b=>b.disabled));
});

test('pantry capacity, persistent hires and discovered chef recipes have separate controls',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels);
 const deposit=buttons(panel.pantry({id:'plot_fridge'}),'bakery_fridge_deposit').find(b=>b.data.item==='dough');assert.deepEqual(deposit.data,{target:'plot_fridge',item:'dough',amount:5});assert.equal(deposit.disabled,false);
 p.inventory.prepared_berry_cookie=3;assert.equal(buttons(panel.pantry({id:'plot_fridge'}),'bakery_fridge_deposit').find(b=>b.data.item==='prepared_berry_cookie').data.amount,3);
 p.bakeryPantry={berry:1000};assert.ok(buttons(panel.pantry({id:'plot_fridge'}),'bakery_fridge_deposit').every(b=>b.disabled));
 let hires=buttons(panel.staff({id:'plot'}),'bakery_hire');assert.deepEqual(hires.map(b=>b.data),[{target:'plot',item:'chef'},{target:'plot',item:'server'},{target:'plot',item:'cashier'}]);assert.ok(hires.every(b=>!b.disabled));
 p.coins=199;p.bakeryStaff.chef=true;hires=buttons(panel.staff({id:'plot'}),'bakery_hire');assert.ok(hires.every(b=>b.disabled));
 const choices=buttons(panel.chef({id:'plot_oven'}),'bakery_chef_recipe_set');assert.deepEqual(choices.map(b=>b.data.item),['sugar','berry_cookie']);assert.equal(choices[0].disabled,true);assert.deepEqual(choices[0].data,{target:'plot_oven',item:'sugar',enabled:false});assert.deepEqual(choices[1].data,{target:'plot_oven',item:'berry_cookie',enabled:true});
 assert.match(panel.staff({id:'plot'}),/pause when you disconnect/);assert.match(panel.pantry({id:'plot_fridge'}),/personal inventory stay there/);
});
test('repeatable ovens and mixers remain purchasable and packed appliances can be placed alongside existing ones',async()=>{
 const {createBakeryPanels}=await modulePromise,{state,p,panel}=fixture(createBakeryPanels);Object.assign(p,{bakeryPlot:'plot',insideBakery:true,bakeryBuildMode:true,bakeryFurniture:[{id:'oven1',kind:'oven'},{id:'mixer1',kind:'mixer'},{id:'display',kind:'display'}]});p.inventory.bakery_oven=1;
 state.snapshot.bakeryFurnitureCatalog=[{id:'oven',name:'Oven',width:2.2,depth:1.4,cost:120,room:'kitchen',core:false},{id:'mixer',name:'Mixer',width:1.6,depth:1.6,cost:90,room:'kitchen',core:false},{id:'display',name:'Display',width:2.8,depth:.8,cost:100,room:'cafe',core:true}];
 const choices=buttons(panel.building(),'bakery-place-choice');assert.deepEqual(choices.map(b=>b.disabled),[false,false,true,false]);assert.deepEqual(choices[3].data,{item:'oven',packed:'bakery_oven'});assert.match(panel.building(),/1 placed/);
});
test('compact customer popup uses the authoritative order and price, with ownership, stock, and reach checks',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels),node={id:'customer',owner:'owner',x:2,z:0};Object.assign(p,{x:0,z:0});p.bakeryBusiness.customers=[{id:node.id,item:'sugar',amount:5,phase:'waiting',price:17}];
 let html=panel.customer(node),serve=buttons(html,'bakery-serve-customer')[0];assert.deepEqual(serve.data,{target:'customer'});assert.equal(serve.label,'Serve · 17 coins');assert.equal(serve.disabled,false);assert.doesNotMatch(html,/shop-list|STOCK YOUR DISPLAY/);
 p.bakeryDisplay.sugar=4;assert.equal(buttons(panel.customer(node),'bakery-serve-customer')[0].disabled,true);p.bakeryDisplay.sugar=5;node.x=4.1;assert.equal(buttons(panel.customer(node),'bakery-serve-customer')[0].disabled,true);node.x=4;node.y=2.6;assert.equal(buttons(panel.customer(node),'bakery-serve-customer')[0].disabled,true);node.y=0;node.owner='other';assert.equal(buttons(panel.customer(node),'bakery-serve-customer').length,0);node.owner='owner';p.bakeryBusiness.customers[0].phase='leaving';assert.equal(buttons(panel.customer(node),'bakery-serve-customer').length,0);
});
test('bakery roles, physical fridge, display capacity and additive achievement formula are explicit',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels);p.bakeryBusiness.achievementCount=3;p.bakeryBusiness.achievementMultiplier=1.3;
 assert.match(panel.staff({id:'plot'}),/Hire server/);assert.match(panel.staff({id:'plot'}),/carry up to 20/);assert.match(panel.staff({id:'plot'}),/1\.3× original order base/);assert.match(panel.staff({id:'plot'}),/1 \+ 0\.10 × N/);assert.match(panel.display({id:'plot_display'}),/5 \/ 300 on display/);assert.match(panel.pantry({id:'plot_fridge'}),/KITCHEN FRIDGE/);assert.match(panel.pantry({id:'plot_fridge'}),/physical fridge/);
});

test('each hired worker gets an independent break action and each role stops hiring at two',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels);p.coins=1000;p.bakeryStaff={chef:true,chef_2:true,server:true,cashier:true,cashier_2:true};p.bakeryStaffBreaks={chef_2:true,cashier:true,cashier_2:true};p.bakeryBusiness.staffLimit=2;p.bakeryBusiness.workers=[{id:'chef',role:'chef',label:'Chef Ada',onBreak:false,status:'Mixing berry cookies'},{id:'chef_2',role:'chef',label:'Chef 2',onBreak:true,status:'On break'}];
 const html=panel.staff({id:'plot'}),hires=buttons(html,'bakery_hire'),breaks=buttons(html,'bakery_staff_break');assert.deepEqual(hires.map(b=>[b.data.item,b.disabled]),[['chef',true],['server',false],['cashier',true]]);assert.match(hires[1].label,/second server/);assert.equal(breaks.length,5);assert.deepEqual(breaks.find(b=>b.data.item==='chef_2').data,{target:'plot',item:'chef_2',enabled:false});assert.equal(breaks.find(b=>b.data.item==='chef_2').label,'Resume work');assert.deepEqual(breaks.find(b=>b.data.item==='chef').data,{target:'plot',item:'chef',enabled:true});assert.match(html,/Chef Ada/);assert.match(html,/Mixing berry cookies/);assert.match(panel.display({id:'display'}),/cashiers are on break/);assert.doesNotMatch(panel.display({id:'display'}),/Cashier serves automatically/);
 p.bakeryStaffBreaks.cashier_2=false;assert.match(panel.display({id:'display'}),/working cashiers serve/);
});

test('chef recipe rotation can add and remove multiple discovered recipes but never the final selection',async()=>{
 const {createBakeryPanels}=await modulePromise,{p,panel}=fixture(createBakeryPanels);p.bakeryStaff={chef:true};p.bakeryChefRecipes=['sugar','berry_cookie'];let html=panel.chef({id:'oven2'}),choices=buttons(html,'bakery_chef_recipe_set');assert.equal(choices.length,2);assert.ok(choices.every(b=>b.data.enabled===false&&!b.disabled));assert.ok(choices.every(b=>b.data.target==='oven2'));assert.match(html,/skipping recipes the fridge cannot currently supply/);assert.match(html,/wait for fridge ingredients/);assert.match(html,/never your personal inventory/);
 p.bakeryChefRecipes=['berry_cookie'];choices=buttons(panel.chef({id:'oven2'}),'bakery_chef_recipe_set');assert.deepEqual(choices.find(b=>b.data.item==='sugar').data,{target:'oven2',item:'sugar',enabled:true});assert.equal(choices.find(b=>b.data.item==='sugar').disabled,false);assert.equal(choices.find(b=>b.data.item==='berry_cookie').disabled,true);p.bakeryStaffBreaks={chef:true};assert.match(panel.chef({id:'oven2'}),/All chefs are on break/);p.bakeryChefRecipes=[];assert.equal(buttons(panel.chef({id:'oven2'}),'bakery_chef_recipe_set').find(b=>b.data.item==='sugar').disabled,true,'legacy single recipe still has a selected entry');
});

test('two displays share capacity and purchased or packed placement obeys the same catalog limit',async()=>{
 const {createBakeryPanels}=await modulePromise,{state,p,panel}=fixture(createBakeryPanels);Object.assign(p,{bakeryPlot:'plot',insideBakery:true,bakeryBuildMode:true,bakeryFurniture:[{id:'display',kind:'display'}]});p.inventory.bakery_display=1;state.snapshot.bakeryFurnitureCatalog=[{id:'display',name:'Display',width:4.2,depth:1.1,cost:100,room:'cafe',core:false,maxCount:2}];
 let html=panel.building();assert.deepEqual(buttons(html,'bakery-place-choice').map(b=>b.disabled),[false,false]);assert.match(html,/1 \/ 2 placed/);assert.match(html,/300 bakes each/);p.bakeryFurniture.push({id:'display2',kind:'display'});assert.ok(buttons(panel.building(),'bakery-place-choice').every(b=>b.disabled));
 p.bakeryBusiness.displayCapacity=600;p.bakeryDisplay={sugar:599};let stock=buttons(panel.display({id:'display2_node'}),'bakery_stock').find(b=>b.data.item==='sugar');assert.deepEqual(stock.data,{target:'display2_node',item:'sugar',amount:1});assert.equal(stock.disabled,false);assert.match(panel.display({id:'display2_node'}),/599 \/ 600 on display/);assert.match(panel.display({id:'display2_node'}),/share one stock/);p.bakeryDisplay.sugar=600;assert.equal(buttons(panel.display({id:'display2_node'}),'bakery_stock').find(b=>b.data.item==='sugar').disabled,true);
});

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {test} = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../web/island-building-panels.js'), 'utf8');
const moduleURL = 'data:text/javascript;base64,' + Buffer.from(source).toString('base64');
async function fixture() {
  const {createIslandBuildingPanels} = await import(moduleURL);
  const me = {id:'owner', island:'owner', coins:0, inventory:{wood:100}, buildMode:true};
  const island = {owner:'owner', builders:{}, chest:{wood:100}, objects:[]};
  const buildCatalog = [
    {id:'wall', name:'Wall', category:'building', cost:{wood:4}, width:4, depth:.3},
    {id:'chair', name:'Chair', category:'decor', cost:{}, coinCost:12, width:1, depth:1},
    {id:'oven', name:'Oven', category:'appliances', cost:{}, coinCost:120, width:2, depth:1.7},
    {id:'flowers', name:'Flower bed', category:'garden', cost:{}, coinCost:15, width:1.8, depth:1.2}
  ];
  const state = {snapshot:{me,island,buildCatalog}}, placements = [], actions = [];
  const button = (label, kind, data={}, disabled=false) => `<button data-action="${kind}" data-item="${data.item||''}" data-packed="${data.packed||''}"${disabled?' disabled':''}>${label}</button>`;
  const panel = createIslandBuildingPanels({state,button,pretty:String,escapeHTML:String,
    shopRow:(_icon,name,description,buttons)=>`<article>${name}<p>${description}</p>${buttons}</article>`,
    closeModal:()=>{},renderModal:()=>{},action:(...args)=>actions.push(args),beginBuild:placement=>placements.push(placement)});
  const catalog = category => {panel.handle('build-category',{category});return panel.building();};
  const place = (html,item,packed='') => html.match(new RegExp(`<button data-action="build-place" data-item="${item}" data-packed="${packed}"[^>]*>`))?.[0];
  return {me,island,state,panel,catalog,place,placements,actions};
}

test('coin catalog prices and affordability use the placing player balance for every converted category', async()=>{
  const {me,island,catalog,place} = await fixture();
  for (const [category,item,price] of [['decor','chair',12],['appliances','oven',120],['garden','flowers',15]]) {
    me.coins = price-1;
    let html = catalog(category);
    assert.match(html,new RegExp(`${price} coins`));
    assert.match(place(html,item),/disabled/,'materials cannot substitute for the coin price');
    me.coins = price;me.inventory = {};
    assert.doesNotMatch(place(catalog(category),item),/disabled/,'coins alone enable a paid furnishing');
    island.owner = 'friend';island.builders.owner = true;
    island.chest = {wood:1000,stone:1000,coins:1000};me.coins = price-1;
    assert.match(place(catalog(category),item),/disabled/,'a guest must pay their own coins');
    me.coins = price;island.chest = {};
    assert.doesNotMatch(place(catalog(category),item),/disabled/,'guest coin purchases do not need shared materials');
    island.builders.owner = false;
    assert.match(place(catalog(category),item),/disabled/,'having coins cannot bypass build permission');
    island.owner = 'owner';
  }
});

test('structural building retains owner inventory and guest shared-material payment', async()=>{
  const {me,island,catalog,place} = await fixture();
  me.coins = 1000;me.inventory = {};
  assert.match(place(catalog('building'),'wall'),/disabled/,'coins cannot replace structural materials');
  me.inventory.wood = 4;
  assert.doesNotMatch(place(catalog('building'),'wall'),/disabled/);
  island.owner = 'friend';island.builders.owner = true;island.chest = {};
  assert.match(place(catalog('building'),'wall'),/disabled/,'guests cannot spend personal structural materials');
  island.chest.wood = 4;me.inventory = {};
  assert.doesNotMatch(place(catalog('building'),'wall'),/disabled/);
});

test('packed purchases remain free and choosing a preview does not send a payment action', async()=>{
  const {me,island,panel,catalog,place,actions,placements} = await fixture();
  me.coins = 0;me.inventory = {build_oven:1};
  let html = catalog('appliances');
  assert.match(place(html,'oven'),/disabled/);
  assert.doesNotMatch(place(html,'oven','build_oven'),/disabled/);
  assert.match(html,/free to place/);
  panel.handle('build-place',{item:'oven',packed:'build_oven'});
  assert.deepEqual(placements,[{item:'oven',packed:'build_oven'}]);
  assert.deepEqual(actions,[],'opening a placement preview must not charge or construct');
  assert.equal(me.coins,0);
  me.buildMode = false;
  assert.match(place(catalog('appliances'),'oven','build_oven'),/disabled/);
  island.owner = 'friend';island.builders.owner = true;me.buildMode = true;
  assert.equal(place(catalog('appliances'),'oven','build_oven'),undefined,'guests cannot place personal packed items on a friend island');
});

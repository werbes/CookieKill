const assert=require('node:assert/strict');

module.exports=function({until,resume,station}){
  async function click(client,action,fields={}){
    const buttons=client.page.locator(`[data-action="${action}"]`);
    const index=await buttons.evaluateAll((nodes,fields)=>nodes.findIndex(node=>{const data=JSON.parse(node.dataset.payload||'{}');return Object.entries(fields).every(([key,value])=>data[key]===value);}),fields);
    assert.ok(index>=0,'missing UI action '+action+' '+JSON.stringify(fields));
    await buttons.nth(index).click();
  }
  async function batch(client,amount){await click(client,'production-batch',{amount,shop:false});}
  async function hold(client,item){
    if(client.snapshot.me.selected===item)return;
    await click(client,'production-hold',{item});
    await until(()=>client.snapshot.me.selected===item,'hold '+item+': '+client.lastError);
  }
  async function collect(client,kind,item){
    const key='bakery_'+kind,amount=client.snapshot.me.production?.[key]?.output?.[item]||0;
    assert.ok(amount>0,'nothing ready at '+kind+' for '+item);
    const before=client.snapshot.me.inventory[item]||0;
    await click(client,'production_collect',{item});
    await until(()=>(client.snapshot.me.inventory[item]||0)===before+Math.min(amount,100),'collect '+item+': '+client.lastError);
  }
  async function mixRecipe(client,recipe,amount){
    await station(client,'mixer',/MAKE PLAIN DOUGH/);await hold(client,'dough');await batch(client,amount);
    await click(client,'production_mix',{item:recipe,amount});
    await until(()=>!!client.snapshot.me.production?.bakery_mixer?.job,'add-in mixing started: '+client.lastError);
    await until(()=>(client.snapshot.me.production?.bakery_mixer?.output?.['prepared_'+recipe]||0)===amount,'add-in mixing finished',Math.max(15000,amount*3000+6000));
    await collect(client,'mixer','prepared_'+recipe);
  }
  async function bake(client,recipe,amount=1){
    const dough=recipe==='sugar'?'dough':'prepared_'+recipe;
    if(recipe!=='sugar'&&(client.snapshot.me.inventory[dough]||0)<amount)await mixRecipe(client,recipe,amount);
    await station(client,'oven',/Hold plain dough/);await hold(client,dough);await batch(client,amount);
    await click(client,'production_bake',{item:dough,amount});
    await until(()=>!!client.snapshot.me.production?.bakery_oven?.job,'oven starts: '+client.lastError);
    await until(()=>(client.snapshot.me.production?.bakery_cooling_table?.output?.[recipe]||0)>=amount,'oven delivers to linked table',amount*5000+12000);
    await station(client,'cooling_table',/FINISHED BAKES/);await collect(client,'cooling_table',recipe);
  }
  return {click,batch,hold,collect,mixRecipe,bake};
};

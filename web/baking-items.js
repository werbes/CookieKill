export const bakingIngredientNames={baking_sugar:'Sugar',butter:'Butter',flour:'Flour',egg:'Egg'};
export const bakingIngredientIcons={baking_sugar:'◇',butter:'▰',flour:'▧',egg:'🥚'};
export function bakingItemName(item,recipeName=id=>id.replaceAll('_',' ')){
  if(bakingIngredientNames[item])return bakingIngredientNames[item];
  if(item==='dough')return 'Cookie dough';
  if(item.startsWith('prepared_'))return 'Prepared '+recipeName(item.slice(9))+' dough';
  return null;
}
export function isDoughItem(item){return item==='dough'||item?.startsWith('prepared_');}
export function createDoughView({THREE,sphere,box},item='dough',scale=1){
  const group=new THREE.Group();group.name='unbaked-dough';group.scale.setScalar(scale);
  const color=item.includes('cactus')?'#c2cf87':item.includes('berry')?'#e4b9a3':'#ead5aa';
  const lump=sphere(.27,color,0,0,0,group,1);lump.scale.set(1.2,.68,.91);
  const fold=sphere(.16,'#f3dfb5',-.14,.09,.04,group,1);fold.scale.set(1,.48,.72);
  if(item.startsWith('prepared_')&&item!=='prepared_sugar')for(let i=0;i<6;i++){const a=i*2.4,bit=sphere(.025,item.includes('berry')?'#ad5966':item.includes('cactus')?'#789c49':'#9e7950',Math.cos(a)*.18,.11+Math.sin(i)*.025,Math.sin(a)*.13,group,0);bit.scale.y=.65;}
  // A dusting of flour and a soft mound make this distinct from a baked cookie.
  for(let i=0;i<3;i++)box(.07,.012,.035,'#f6e7c6',-.13+i*.105,.15,.015+i*.025,group);
  return group;
}

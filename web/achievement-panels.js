export function createAchievementPanels({state,button,escapeHTML}){
  const esc=value=>escapeHTML(String(value??''));
  const catalog=()=>state.snapshot?.achievementCatalog||[],hatsCatalog=()=>state.snapshot?.hatCatalog||[],me=()=>state.snapshot?.me||{};
  function hats(draft){
    const p=me();
    return `<div class="section-label">ACHIEVEMENT HATS <span>Earn an achievement, then wear its hat</span></div><div class="hat-options">${button(!draft.hat?'✓ No hat':'No hat','avatar-hat',{hat:''})}${hatsCatalog().map(h=>{
      const unlocked=!!p.hats?.[h.id],achievement=catalog().find(a=>a.id===h.achievement);
      return `<div class="hat-option${unlocked?'':' locked'}">${button((draft.hat===h.id?'✓ ':'')+h.name,'avatar-hat',{hat:h.id},!unlocked)}<span>${esc(unlocked?'Unlocked':achievement?.description||'Earn the matching achievement.')}</span></div>`;
    }).join('')}</div>`;
  }
  function preview(id){return hatsCatalog().find(h=>h.id===id)?.icon||'';}
  function gallery(){
    const p=me(),earned=p.achievements||{},valid=new Set(catalog().map(a=>a.id)),hung=new Set((p.hungAchievements||[]).filter(id=>valid.has(id)&&earned[id]));
    const count=hung.size,factor=(1+count/10).toFixed(1),plot=state.snapshot?.nodes?.find(n=>n.id===p.bakeryPlot&&n.kind==='bakery_plot'&&n.owner===p.id);
    const near=state.connected!==false&&p.bakery&&!!plot&&!p.island&&!p.inCave&&Math.abs(p.y||0)<=2&&Math.hypot(p.x-plot.x,p.z-plot.z)<=10;
    const total=catalog().filter(a=>earned[a.id]).length;
    return `<div class="section-label">YOUR ACHIEVEMENTS <span>${total} / ${catalog().length} earned · ${count} hanging</span></div><p class="field-hint">Achievements and their hats stay earned permanently. Hang each achievement once on your bakery's gallery wall. Every distinct earned frame adds 10% of the original customer price.</p><div class="notice good"><strong>Achievement price factor: ${factor}×</strong><p>1 + 0.10 × ${count} = ${factor}× the original price. Four frames give 1.4×. Other bakery bonuses are added separately; the combined achievement bonus rounds down once to whole coins.</p></div>${near?'':'<p class="field-hint">Visit your own bakery sign or interior to hang or remove frames.</p>'}<div class="shop-list">${catalog().map(a=>{
      const unlocked=!!earned[a.id],displayed=hung.has(a.id),hat=hatsCatalog().find(h=>h.id===a.hat);
      return `<div class="notice${unlocked?' good':''}"><strong>${esc(a.icon)} ${esc(a.name)}</strong><p>${esc(a.description)}</p><p class="field-hint">${unlocked?'Earned':'Locked'} · ${esc(hat?.name||'Achievement hat')}${displayed?' · Hanging in your bakery':''}</p><div class="item-actions">${button(displayed?'Take down frame':'Hang in bakery',displayed?'unhang_achievement':'hang_achievement',{item:a.id},!near||!unlocked,!displayed&&unlocked)}</div></div>`;
    }).join('')}</div>`;
  }
  return {hats,preview,gallery};
}

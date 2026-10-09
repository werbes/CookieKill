const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),read=f=>fs.readFileSync(path.join(root,f),'utf8'),url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const shared=url(read('web/camel-race-state.js')),resolve=s=>s.replaceAll('./camel-race-state.js',shared),building=url(resolve(read('web/island-building-panels.js')));
const modules=Promise.all([import(url(resolve(read('web/island-panels.js')).replace('./island-building-panels.js',building))),import(url(resolve(read('web/camel-race-panels.js'))))]);

test('Teleport combines home travel, unlocked friend visits, and casual races without requiring a lollipop',async()=>{
 const [{createIslandPanels},{createCamelRacePanels}]=await modules,buttons=[];
 const me={id:'one',coins:0,homeIsland:{owner:'one'},friendsTeleport:false},state={connected:true,snapshot:{me,friends:[{id:'friend',callName:'Pal',username:'pal',ownsIsland:true,visitors:true}],camelRace:{casualLobbies:[]}}};
 const deps={state,escapeHTML:String,pretty:String,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;},shopRow:(_icon,name,desc,controls)=>name+desc+controls};
 const races=createCamelRacePanels(deps),panels=createIslandPanels({...deps,renderCasualRaces:()=>races.casualMenu()});
 let html=panels.render('teleport');assert.match(html,/Go to my island/);assert.match(html,/blue lollipop/);assert.match(html,/Host a playful race/);assert.ok(!buttons.find(b=>b.action==='race_casual_host').disabled);assert.ok(!buttons.some(b=>b.action==='visit_island'));
 me.friendsTeleport=true;buttons.length=0;html=panels.render('teleport');assert.ok(buttons.some(b=>b.action==='visit_island'&&b.data.target==='friend'&&!b.disabled));assert.match(html,/Pal/);
 me.island='camel_casual:one';Object.assign(state.snapshot.camelRace,{mode:'casual',phase:'lobby',hostId:'one'});buttons.length=0;html=panels.render('teleport');assert.ok(buttons.some(b=>b.action==='tab'&&b.data.tab==='race'));assert.ok(!buttons.some(b=>['teleport_home','teleport_main','race_casual_host'].includes(b.action)&&!b.disabled));
});

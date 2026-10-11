const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const {test} = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../web/game.js'), 'utf8');
const interactions = source.slice(source.indexOf('function nearby(){'), source.indexOf('function cancelBuild('));

function fixture(animal) {
  const elements = new Map(), actions = [], modals = [], messages = [];
  const state = {locked: true, snapshot: {me: {id: 'swimmer', x: -80, y: -.6, z: 100, reputation: {}}, nodes: [], animals: [animal]}};
  const context = vm.createContext({
    state,
    $: id => {if (!elements.has(id)) elements.set(id, {}); return elements.get(id);},
    isCamelRaceIsland: () => false,
    heldTreatTarget: () => null,
    heldCamelTreat: () => null,
    pretty: value => String(value).replaceAll('_', ' ').replace(/\b\w/g, s => s.toUpperCase()),
    itemName: () => 'Sugar cookie',
    action: (name, payload) => actions.push({name, ...payload}),
    openModal: (name, target) => modals.push({name, target}),
    toast: text => messages.push(text),
  });
  vm.runInContext(interactions, context);
  return {state, elements, actions, modals, messages, update: () => context.updateInteraction(), interact: () => context.interact()};
}

const animal = (species, extra = {}) => ({id: species + '_1', species, x: -79, y: .02, z: 100, health: 50, need: '', egg: false, ...extra});

test('nearby sea animals without a node kind keep interaction frames running', () => {
  for (const species of ['fish', 'turtle', 'dolphin', 'sea_lion', 'shark', 'whale']) {
    const f = fixture(animal(species));
    assert.equal(f.state.snapshot.animals[0].kind, undefined, 'animals use species, unlike resource nodes');
    for (let frame = 0; frame < 5; frame++) {
      f.state.snapshot.me.x += .1;
      assert.doesNotThrow(f.update, species + ' interaction frame ' + frame);
      assert.equal(f.elements.get('interaction-prompt').hidden, false);
      assert.equal(f.state.nearest.isAnimal, true);
    }
    f.interact();
    assert.deepEqual(f.actions, [{name: 'interact', target: species + '_1'}]);
  }
});

test('wounded fish and turtles show rescue help and open the animal menu', () => {
  for (const species of ['fish', 'turtle']) {
    const f = fixture(animal(species, {need: 'wounded'}));
    f.update();
    assert.equal(f.elements.get('interaction-label').textContent, 'Help the wounded ' + species);
    assert.match(f.elements.get('interaction-detail').textContent, /Offer one sugar cookie/);
    f.interact();
    assert.equal(f.modals.length, 1);
    assert.equal(f.modals[0].name, 'animal');
    assert.equal(f.modals[0].target.id, species + '_1');
    assert.equal(f.actions.length, 0);
  }
});

test('trapped turtles and animal eggs retain their interaction behavior', () => {
  const trapped = fixture(animal('turtle', {need: 'trapped'}));
  trapped.update();
  assert.equal(trapped.elements.get('interaction-label').textContent, 'Free the turtle');
  trapped.interact();
  assert.deepEqual(trapped.actions, [{name: 'interact', target: 'turtle_1'}]);
  for (const species of ['fish', 'turtle']) {
    const egg = fixture(animal(species, {egg: true, hatchIn: 43}));
    egg.update();
    assert.equal(egg.elements.get('interaction-label').textContent, 'A ' + species + ' egg');
    assert.match(egg.elements.get('interaction-detail').textContent, /43 seconds undisturbed/);
    egg.interact();
    assert.match(egg.messages[0], /Stay at least two metres away/);
    assert.equal(egg.actions.length, 0);
  }
});

test('practice equipment still uses its node kind for prompts and actions', () => {
  const f = fixture(animal('fish', {x: -90}));
  f.state.snapshot.nodes = [{id: 'practice_bench_press', kind: 'practice_bench_press', x: -79, z: 100, available: true, label: 'Bench press'}];
  f.update();
  assert.equal(f.elements.get('interaction-label').textContent, 'Bench press');
  f.interact();
  assert.deepEqual(f.actions, [{name: 'practice_exercise', target: 'practice_bench_press'}]);
});

export const isCamelRaceIsland = island => island === 'camel_race' || typeof island === 'string' && island.startsWith('camel_casual:');

export function camelRaceReminder(snapshot) {
  const start = snapshot?.camelRace?.reminderStartAt || 0;
  const remaining = start - (snapshot?.time || 0);
  return start > 0 && remaining > 0 && remaining <= 120 ? remaining : 0;
}

import { describe as group, expect, test } from 'vitest';
import { describe, fraction, snap, stopIndex, stopLabel, stops } from './scale';

group('scale', () => {
  test('saaty9 has 17 stops from 1/9 to 9', () => {
    const s = stops('saaty9');
    expect(s).toHaveLength(17);
    expect(s[0]).toBeCloseTo(1 / 9);
    expect(s[8]).toBe(1);
    expect(s[16]).toBe(9);
  });

  test('snap rounds on a log scale, like the server', () => {
    expect(snap(2.6, 'saaty9')).toBe(3);
    expect(snap(0.826, 'saaty9')).toBe(1);
    expect(snap(3, 'classic5')).toBe(5);
    expect(snap(0.3, 'classic5')).toBe(0.2);
    expect(stopIndex(1 / 3, 'saaty9')).toBe(6);
  });

  test('describe phrases criteria and alternatives differently', () => {
    expect(describe(5, 'saaty9', 'criteria', 'Cost', 'Speed')).toBe('Cost matters strongly more than Speed');
    expect(describe(1 / 3, 'saaty9', 'alternatives', 'Alpha', 'Beta', 'Price')).toBe('Beta is moderately better than Alpha on Price');
    expect(describe(1, 'saaty9', 'criteria', 'Cost', 'Speed')).toBe('Cost and Speed matter equally');
    expect(describe(10, 'classic5', 'alternatives', 'A', 'B')).toBe('A is much better than B');
  });

  test('fraction writes reciprocals as people do', () => {
    expect(fraction(5)).toBe('5');
    expect(fraction(0.2)).toBe('1/5');
    expect(fraction(1 / 7)).toBe('1/7');
    expect(fraction(1)).toBe('1');
  });
});

test('stop labels name the anchor strengths on each scale', () => {
  const saaty = stops('saaty9').map((v) => stopLabel(v, 'saaty9'));
  expect(saaty).toEqual(['Extreme', '', 'Very strong', '', 'Strong', '', 'Moderate', '', 'Equal', '', 'Moderate', '', 'Strong', '', 'Very strong', '', 'Extreme']);
  expect(stops('classic5').map((v) => stopLabel(v, 'classic5'))).toEqual(['Much', 'Somewhat', 'Equal', 'Somewhat', 'Much']);
});

test('a previewed judgment reads as a possibility', () => {
  expect(describe(5, 'saaty9', 'criteria', 'Cost', 'Speed', '', true)).toBe('Cost would matter strongly more than Speed');
  expect(describe(1, 'saaty9', 'criteria', 'Cost', 'Speed', '', true)).toBe('Cost and Speed would matter equally');
  expect(describe(1 / 3, 'saaty9', 'alternatives', 'A', 'B', 'Price', true)).toBe('B would be moderately better than A on Price');
  expect(describe(1, 'classic5', 'alternatives', 'A', 'B', '', true)).toBe('A and B would be equal');
});

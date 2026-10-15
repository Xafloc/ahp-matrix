import { describe as group, expect, test } from 'vitest';
import { describe, fraction, snap, stopIndex, stops } from './scale';

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

import { expect, test } from 'vitest';
import { fixture } from './fixture.test-helper';
import { explain } from './problems';

test('validation problems become plain sentences that say what to fix', () => {
  const p = fixture();
  p.alternatives[1].name = '';
  p.evaluators[0].name = '';
  expect(
    explain(
      [
        { field: 'name', message: 'is required' },
        { field: 'alternatives[1].name', message: 'is required' },
        { field: 'criteria[3].name', message: 'is required' },
        { field: 'evaluators[1].weight', message: 'must be greater than 0' },
        { field: 'evaluators[0].name', message: 'is required' },
        { field: 'pairwise[4].value', message: 'must be a positive number' },
      ],
      p,
    ),
  ).toEqual([
    'The decision needs a name.',
    'Option 2 needs a name.',
    'Cost needs a name.',
    "Bob's weight must be more than 0.",
    'Person 1 needs a name.',
    'pairwise[4].value must be a positive number.',
  ]);
});

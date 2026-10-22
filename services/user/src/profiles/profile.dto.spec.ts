import { plainToInstance } from 'class-transformer';
import { validateSync } from 'class-validator';

import { UpdateProfileDto } from './profile.dto';

const errorsFor = (body: object): string[] =>
  validateSync(plainToInstance(UpdateProfileDto, body)).map((e) => e.property);

describe('UpdateProfileDto', () => {
  it('rejects a null username instead of passing it to the service', () => {
    expect(errorsFor({ username: null })).toEqual(['username']);
  });

  it('accepts an absent username', () => {
    expect(errorsFor({ bio: 'hello' })).toEqual([]);
  });

  it('still lets null clear display_name and bio', () => {
    expect(errorsFor({ display_name: null, bio: null })).toEqual([]);
  });

  it('accepts a username string', () => {
    expect(errorsFor({ username: 'awa.kone' })).toEqual([]);
  });
});

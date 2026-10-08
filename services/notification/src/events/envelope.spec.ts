import { uuidv7 } from '../common/uuid';
import { InvalidEnvelopeError, decodeEnvelope, dlqTopic, newEnvelope } from './envelope';

describe('event envelopes', () => {
  it('round-trips an envelope compatible with shared-go/events', () => {
    const at = new Date('2026-09-30T10:00:00.000Z');
    const env = newEnvelope('poro.user.creator.activated', 1, 'subject-1', { user_id: 'u' }, at);
    expect(env).toMatchObject({
      version: 1,
      source: 'poro-notification',
      occurred_at: '2026-09-30T10:00:00.000Z',
    });
    expect(decodeEnvelope(Buffer.from(JSON.stringify(env)))).toEqual(env);
    expect(dlqTopic('poro.auth.user.created')).toBe('poro.auth.user.created.dlq');
  });

  it('decodes Go envelopes with nanosecond timestamps', () => {
    const goJson = JSON.stringify({
      id: uuidv7(),
      type: 'poro.auth.user.created',
      version: 1,
      source: 'poro-auth',
      subject: 'u1',
      occurred_at: '2026-09-30T10:00:00.123456789Z',
      data: { user_id: 'u1' },
    });
    expect(decodeEnvelope(Buffer.from(goJson)).source).toBe('poro-auth');
  });

  it.each(['poro.video.ready', 'poro.social.like.created'])('accepts the type %s', (type) => {
    const env = { ...valid, type };
    expect(decodeEnvelope(Buffer.from(JSON.stringify(env))).type).toBe(type);
  });

  const valid = {
    id: uuidv7(),
    type: 'poro.auth.user.created',
    version: 1,
    source: 'poro-auth',
    subject: 's',
    occurred_at: '2026-09-30T10:00:00Z',
    data: {},
  };
  it.each([
    ['null value', null],
    ['not json', Buffer.from('{')],
    ['a number', Buffer.from('42')],
    ['missing id', Buffer.from(JSON.stringify({ ...valid, id: undefined }))],
    ['bad type', Buffer.from(JSON.stringify({ ...valid, type: 'user.created' }))],
    ['too short a type', Buffer.from(JSON.stringify({ ...valid, type: 'poro.video' }))],
    ['too long a type', Buffer.from(JSON.stringify({ ...valid, type: 'poro.a.b.c.d' }))],
    ['zero version', Buffer.from(JSON.stringify({ ...valid, version: 0 }))],
    ['no source', Buffer.from(JSON.stringify({ ...valid, source: '' }))],
    ['no subject', Buffer.from(JSON.stringify({ ...valid, subject: '' }))],
    ['bad date', Buffer.from(JSON.stringify({ ...valid, occurred_at: 'yesterday' }))],
    ['array data', Buffer.from(JSON.stringify({ ...valid, data: [] }))],
  ])('rejects %s', (_name, value) => {
    expect(() => decodeEnvelope(value)).toThrow(InvalidEnvelopeError);
  });
});

import { CompressionCodecs, CompressionTypes } from 'kafkajs';

import { registerSnappyCodec } from '../src/events/snappy-codec';

describe('Snappy codec for KafkaJS', () => {
  it('round-trips bytes that franz-go would compress', async () => {
    registerSnappyCodec();
    const codec = CompressionCodecs[CompressionTypes.Snappy]();
    const payload = Buffer.from('{"type":"poro.shop.product.updated","v":1}');
    const compressed = await codec.compress({ buffer: payload });
    expect(compressed.length).toBeGreaterThan(0);
    const plain = await codec.decompress(compressed);
    expect(Buffer.from(plain).toString()).toBe(payload.toString());
  });
});

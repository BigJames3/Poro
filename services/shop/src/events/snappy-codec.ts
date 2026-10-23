import { CompressionCodecs, CompressionTypes } from 'kafkajs';

// Nest emits CommonJS: import.meta is forbidden. snappyjs is a CJS package.
const snappyjs = require('snappyjs') as {
  compress: (input: Buffer | Uint8Array | ArrayBuffer) => Uint8Array;
  uncompress: (input: Buffer | Uint8Array | ArrayBuffer) => Uint8Array;
};

/** Lets KafkaJS read batches produced by franz-go (default Snappy). */
export function registerSnappyCodec(): void {
  CompressionCodecs[CompressionTypes.Snappy] = () => ({
    async compress(encoder: { buffer: Buffer }) {
      return Buffer.from(snappyjs.compress(encoder.buffer));
    },
    async decompress(buffer: Buffer) {
      return Buffer.from(snappyjs.uncompress(buffer));
    },
  });
}

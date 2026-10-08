import { Logger } from '@nestjs/common';

// Error paths are asserted through responses; their log lines only add noise.
Logger.overrideLogger(false);

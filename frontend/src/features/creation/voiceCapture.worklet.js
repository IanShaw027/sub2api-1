class VoiceCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super()
    this.samples = new Int16Array(2048)
    this.length = 0
    this.energy = 0
  }

  process(inputs) {
    const input = inputs[0]?.[0]
    if (!input) return true
    for (const value of input) {
      const sample = Math.max(-1, Math.min(1, value))
      this.samples[this.length++] = sample < 0 ? sample * 32768 : sample * 32767
      this.energy += sample * sample
      if (this.length === this.samples.length) {
        this.port.postMessage({ pcm: this.samples.buffer, level: Math.sqrt(this.energy / this.length) }, [this.samples.buffer])
        this.samples = new Int16Array(2048)
        this.length = 0
        this.energy = 0
      }
    }
    return true
  }
}

registerProcessor('creation-voice-capture', VoiceCaptureProcessor)

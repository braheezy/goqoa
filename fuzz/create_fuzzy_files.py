#!/usr/bin/env python

import struct
import random
import os

COUNT=20

def generate_random_history_weights():
    return [random.randint(-32768, 32767) for _ in range(4)]

def generate_qoa_slice():
    # Generate a slice with random 20 samples of audio data
    return os.urandom(8)  # 8 bytes for 20 samples, assuming random data is acceptable

def generate_qoa_file(samples, num_channels, samplerate):
    file_header = struct.pack(">4sI", b"qoaf", samples)

    samples_per_frame = 256 * 20
    num_frames = (samples + samples_per_frame - 1) // samples_per_frame

    frames = []
    for _ in range(num_frames):
        frame_samples = min(samples, samples_per_frame)
        # Ensure the calculation of frame_size does not exceed 'H' limits
        frame_size = 8 + num_channels * (16 + 256 * 8)  # Adjusted calculation
        if frame_size > 65535:
            raise ValueError("Frame size exceeds allowable range for 'H' format.")

        # Pack the samplerate as three separate bytes
        sr_byte1 = (samplerate >> 16) & 0xFF
        sr_byte2 = (samplerate >> 8) & 0xFF
        sr_byte3 = samplerate & 0xFF

        frame_header = struct.pack(">BBBBHH", num_channels, sr_byte1, sr_byte2, sr_byte3, frame_samples, frame_size)

        lms_states = b''
        for _ in range(num_channels):
            history = generate_random_history_weights()
            weights = generate_random_history_weights()
            lms_state = struct.pack(">8h", *history, *weights)
            lms_states += lms_state

        slices = b''.join(generate_qoa_slice() for _ in range(256 * num_channels))

        frames.append(frame_header + lms_states + slices)

    qoa_content = file_header + b''.join(frames)
    return qoa_content



def save_qoa_file(filename, content):
    with open(filename, 'wb') as f:
        f.write(content)

def main():
    for _ in range(COUNT):
        samples = random.randint(20, 1000)  # Choose a random number of samples
        num_channels = random.randint(1, 8)  # Choose a random number of channels
        samplerate = random.randint(1, 16777215)  # Random sample rate within valid range
        qoa_content = generate_qoa_file(samples, num_channels, samplerate)
        filename = f"fuzz_qoa_{samples}_{num_channels}_{samplerate}.qoa"
        save_qoa_file(filename, qoa_content)
        print(f"Generated {filename}")

if __name__ == "__main__":
    main()

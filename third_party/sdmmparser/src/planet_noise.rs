use noise::{NoiseFn, Perlin};

/// Matches rust-g 4.0 noise_get_at_coordinates (noise 0.9, including scaling).
/// Batch one lattice across the FFI boundary instead of calling per tile.
#[no_mangle]
pub unsafe extern "C" fn SdmmPlanetNoise(seed: u32, x: f64, y: f64, step: f64,
    width: u32, height: u32, output: *mut f64) {
    if output.is_null() || width == 0 || height == 0 || width > 1024 || height > 1024 {
        return;
    }
    let generator = Perlin::new(seed);
    let result = std::slice::from_raw_parts_mut(output, (width * height) as usize);
    for row in 0..height {
        for col in 0..width {
            let n = generator.get([x + col as f64 * step, y + row as f64 * step]);
            result[(row * width + col) as usize] = ((n * 2.0_f64.sqrt() + 1.0) / 2.0).clamp(0.0, 1.0);
        }
    }
}

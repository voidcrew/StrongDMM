extern const char* SdmmParseEnvironment(const char* nativePath);
extern const char* SdmmParsePreviewEnvironment(const char* nativePath);
extern const char* SdmmParseIconMetadata(const char* nativePath);
extern void SdmmFreeStr(char* nativeStr);
extern void SdmmPlanetNoise(unsigned int seed, double x, double y, double step, unsigned int width, unsigned int height, double *output);

#!/bin/sh
# Refaz ialocal/motor/llama-win-x64.zip: o llama-server do llama.cpp para
# Windows, compilado com MinGW-w64 (no Linux), que não precisa do Visual C++
# Redistributable. Precisa de: git, cmake, ninja, g++-mingw-w64-x86-64-posix.
set -e
V=b12428
D=${TMPDIR:-/tmp}/nep-motor
rm -rf "$D" && mkdir -p "$D"
git clone -q --depth 1 --branch $V https://github.com/ggml-org/llama.cpp "$D/src"
# os cabeçalhos do MinGW não têm THREAD_POWER_THROTTLING_STATE
python3 - "$D/src/ggml/src/ggml-cpu/ggml-cpu.c" <<'PY'
import sys
p=sys.argv[1]; s=open(p).read()
c='''#if defined(_WIN32) && defined(__MINGW32__) && !defined(THREAD_POWER_THROTTLING_CURRENT_VERSION)
typedef struct _THREAD_POWER_THROTTLING_STATE { ULONG Version; ULONG ControlMask; ULONG StateMask; } THREAD_POWER_THROTTLING_STATE;
#define THREAD_POWER_THROTTLING_CURRENT_VERSION 1
#define THREAD_POWER_THROTTLING_EXECUTION_SPEED 0x1
#endif
'''
i=s.index('THREAD_POWER_THROTTLING_STATE t;'); j=s.rfind('\nstatic bool ',0,i)
open(p,'w').write(s[:j+1]+c+s[j+1:])
PY
cat > "$D/mingw.cmake" <<'T'
set(CMAKE_SYSTEM_NAME Windows)
set(CMAKE_SYSTEM_PROCESSOR x86_64)
set(CMAKE_C_COMPILER x86_64-w64-mingw32-gcc-posix)
set(CMAKE_CXX_COMPILER x86_64-w64-mingw32-g++-posix)
set(CMAKE_RC_COMPILER x86_64-w64-mingw32-windres)
set(CMAKE_FIND_ROOT_PATH /usr/x86_64-w64-mingw32)
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
T
export HF_UI_VERSION=$V
cmake -S "$D/src" -B "$D/b" -G Ninja -DCMAKE_TOOLCHAIN_FILE="$D/mingw.cmake" -DCMAKE_BUILD_TYPE=Release -DLLAMA_BUILD_NUMBER=${V#b} \
  -DBUILD_SHARED_LIBS=ON -DGGML_BACKEND_DL=ON -DGGML_CPU_ALL_VARIANTS=ON -DGGML_NATIVE=OFF -DGGML_OPENMP=OFF \
  -DLLAMA_CURL=OFF -DLLAMA_OPENSSL=OFF -DLLAMA_BUILD_TESTS=OFF -DLLAMA_BUILD_EXAMPLES=OFF
ninja -C "$D/b" llama-server
cd "$D/b/bin"
cp /usr/lib/gcc/x86_64-w64-mingw32/*-posix/libstdc++-6.dll /usr/lib/gcc/x86_64-w64-mingw32/*-posix/libgcc_s_seh-1.dll /usr/x86_64-w64-mingw32/lib/libwinpthread-1.dll .
x86_64-w64-mingw32-strip --strip-unneeded *.exe *.dll
cp "$D/src/LICENSE" LICENSE-llama.cpp.txt
printf 'llama-server (llama.cpp %s, licença MIT) compilado para o Nepenthes com MinGW-w64.\nlibstdc++-6.dll, libgcc_s_seh-1.dll: GCC, GPL 3 com a exceção de biblioteca de execução (GCC Runtime Library Exception).\nlibwinpthread-1.dll: MinGW-w64 winpthreads, licença MIT/BSD.\nNão precisa do Visual C++ Redistributable.\n' $V > LEIA-ME-MOTOR.txt
rm -f "$OLDPWD/ialocal/motor/llama-win-x64.zip"
zip -q -9 "$OLDPWD/ialocal/motor/llama-win-x64.zip" *.exe *.dll *.txt

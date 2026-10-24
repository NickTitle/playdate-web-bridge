/* Actual SDL disk-backend PCM tap. Opt-in LD_PRELOAD component. Output uses an empty memfd; nonblocking datagrams cannot grow
 * files or wait for a receiver. SFPC v1 packet: magic, little-endian byte count,
 * monotonic nanoseconds, then the calibrated 44100Hz stereo signed-16 LE PCM.
 * Timestamp denotes callback write time, not browser playback time.
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <sys/mman.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include <time.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>
static FILE *audio;
static int endpoint = -1;
static struct sockaddr_un destination;
static FILE *open_audio(const char *path, const char *mode, const char *symbol) {
 FILE *(*real)(const char *,const char *) = dlsym(RTLD_NEXT,symbol);
 const char *target = getenv("SDL_DISKAUDIOFILE");
 const char *socket_path = getenv("PLAYDATE_BRIDGE_AUDIO_SOCKET");
 if (!target || !socket_path || strcmp(path,target) || !strchr(mode,'w')) return real(path,mode);
 /* Fail closed if configuration is invalid: never create an unbounded file. */
 int fd = memfd_create("playdate-bridge-audio", MFD_CLOEXEC|MFD_ALLOW_SEALING);
 if (fd >= 0 && fcntl(fd,F_ADD_SEALS,F_SEAL_GROW|F_SEAL_SHRINK|F_SEAL_WRITE|F_SEAL_SEAL)<0) { close(fd); return NULL; }
 FILE *f = fd < 0 ? NULL : fdopen(fd,mode);
 if (!f && fd >= 0) close(fd);
 audio = f;
 if (endpoint >= 0) close(endpoint);
 endpoint = -1;
 if (strlen(socket_path) >= sizeof(destination.sun_path)) return f;
 memset(&destination,0,sizeof(destination));
 destination.sun_family = AF_UNIX;
 strcpy(destination.sun_path,socket_path);
 endpoint = socket(AF_UNIX,SOCK_DGRAM|SOCK_NONBLOCK|SOCK_CLOEXEC,0);
 return f;
}
FILE *fopen(const char *path,const char *mode) { return open_audio(path,mode,"fopen"); }
FILE *fopen64(const char *path,const char *mode) { return open_audio(path,mode,"fopen64"); }
static void little(unsigned char *p,uint64_t n,int bytes) {
 for(int i=0;i<bytes;i++) { p[i]=(unsigned char)n; n >>= 8; }
}
size_t fwrite(const void *ptr,size_t size,size_t count,FILE *f) {
 size_t (*real)(const void *,size_t,size_t,FILE *) = dlsym(RTLD_NEXT,"fwrite");
 /* SDL requires a regular file or FIFO. The empty memfd satisfies that
  * check; intercepted PCM never gets written to it. */
 size_t written = f == audio ? count : real(ptr,size,count,f);
 if (f == audio && endpoint >= 0 && size && written <= 65536/size) {
  size_t bytes = size*written;
  /* Reject oversized blocks; never truncate a PCM block silently. */
  unsigned char packet[65552];
  struct timespec now;
  clock_gettime(CLOCK_MONOTONIC,&now);
  memcpy(packet,"SFPC",4);
  little(packet+4,bytes,4);
  little(packet+8,(uint64_t)now.tv_sec*1000000000ULL+now.tv_nsec,8);
  memcpy(packet+16,ptr,bytes);
  /* EAGAIN/ENOENT: drop this block. Never stall the simulator audio thread. */
  (void)sendto(endpoint,packet,bytes+16,MSG_DONTWAIT|MSG_NOSIGNAL,
    (const struct sockaddr *)&destination,sizeof(destination));
 }
 return written;
}
int fclose(FILE *f) {
 int (*real)(FILE *) = dlsym(RTLD_NEXT,"fclose");
 if(f == audio) { audio=NULL; if(endpoint>=0) close(endpoint); endpoint=-1; }
 return real(f);
}

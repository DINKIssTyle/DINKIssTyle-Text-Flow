#ifndef DKST_SYSTEMTRAY_DARWIN_H
#define DKST_SYSTEMTRAY_DARWIN_H

void *dkstSystemTrayCreate(const unsigned char *activeIconBytes, int activeIconLength,
                           const unsigned char *pausedIconBytes, int pausedIconLength);
void dkstSystemTrayUpdateState(void *tray, int flowPaused, int running,
                               int ocrEnabled);
void dkstSystemTrayDestroy(void *tray);

#endif

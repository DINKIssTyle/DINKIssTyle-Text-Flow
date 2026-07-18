#ifndef DKST_SYSTEMTRAY_DARWIN_H
#define DKST_SYSTEMTRAY_DARWIN_H

void *dkstSystemTrayCreate(const unsigned char *iconBytes, int iconLength);
void dkstSystemTrayDestroy(void *tray);

#endif

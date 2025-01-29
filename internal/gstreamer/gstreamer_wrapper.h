#ifndef GSTREAMER_WRAPPER_H
#define GSTREAMER_WRAPPER_H
#include <gst/gstpipeline.h>

extern void onNewFrame(void *buffer, int buffer_size, int duration, int id);

extern void onBusMessage(char *message_type, char *message, int id);

void gstreamer_init();

void gstreamer_start_main_loop();
void gstreamer_stop_main_loop();

void gstreamer_deinit();

void* gstreamer_prepare_pipelines(const char *pipeline_str, int id);

void gstreamer_dispose_pipeline(void *state);

void gstreamer_push_buffer(void *state, void *buffer, size_t buffer_size, unsigned long duration);

void gstreamer_start_pipeline(void *state);

void gstreamer_stop_pipeline(void *state);


#endif //GSTREAMER_WRAPPER_H

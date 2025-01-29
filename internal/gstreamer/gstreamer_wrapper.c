#include "gstreamer_wrapper.h"
#include <gst/gst.h>

#include <gst/app/gstappsrc.h>
#include <gst/app/gstappsink.h>

void gstreamer_on_new_sample(GstAppSink *appsink, void* user_data);

gboolean gstreamer_bus_watch(GstBus *bus, GstMessage *msg, void *user_data);
static GMainLoop *loop = NULL;

typedef struct {
    GstElement *pipeline;
    guint bus_watch_id;
    int id;
} t_gstreamer_wrapper;

void gstreamer_init() {
    int argc = 0;
    gst_init(&argc, NULL);
}

void gstreamer_deinit() {
    gst_deinit();
}

gboolean gstreamer_bus_watch(GstBus *bus, GstMessage *msg, void* user_data) {
    switch (GST_MESSAGE_TYPE(msg)) {
        case GST_MESSAGE_ERROR: {
            GError *error = NULL;
            gchar *debug_info = NULL;
            gst_message_parse_error(msg, &error, &debug_info);
            t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) user_data;
            if (self) {
                onBusMessage("ERROR", error->message, self->id);
                g_free(debug_info);
                exit(1);
                //g_main_loop_quit(self->loop);
            }
            break;
        }
        case GST_MESSAGE_EOS: {
            t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) user_data;
            if (self) {
                onBusMessage("EOS", "OK", self->id);
                if (self->pipeline) {
                    gst_element_set_state(self->pipeline, GST_STATE_NULL);
                }
            }
            break;
        }
        default: {
            break;
        }
    }
    return TRUE;
}

void gstreamer_on_new_sample(GstAppSink *appsink, void* user_data) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) user_data;
    GstSample *sample = gst_app_sink_pull_sample(appsink);
    GstBuffer *buffer = gst_sample_get_buffer(sample);
    GstMapInfo info;
    if (gst_buffer_map(buffer, &info, GST_MAP_READ)) {
        onNewFrame(info.data, info.size, buffer->duration, self->id);
        gst_buffer_unmap(buffer, &info);
    }
    gst_sample_unref(sample);
}

void *gstreamer_prepare_pipelines(const char *pipeline_str, int id) {
    GstElement *pipeline = NULL;
    GstBus *bus = NULL;
    GstMessage *msg = NULL;
    t_gstreamer_wrapper *self = malloc(sizeof(t_gstreamer_wrapper));
    GError *err = NULL;
    pipeline = gst_parse_launch(pipeline_str, &err);
    if (err != NULL) {
        free(self);
        g_print("Failed to parse pipeline %s: %s", pipeline_str, err->message);
        g_error_free(err);
        return NULL;
    }
    bus = gst_element_get_bus(pipeline);
    self->bus_watch_id = gst_bus_add_watch(bus, gstreamer_bus_watch, self);
    self->id = id;
    gst_object_unref(bus);
    GstElement *app_sink = gst_bin_get_by_name(GST_BIN(pipeline), "sink");
    if (app_sink != NULL) {
        g_signal_connect(app_sink, "new-sample", G_CALLBACK(gstreamer_on_new_sample), self);
    }
    g_debug("prepared");
    self->pipeline = pipeline;
    return self;
}

void gstreamer_dispose_pipeline(void *pipeline) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) pipeline;

    g_source_remove(self->bus_watch_id);
    free(self);
}

void gstreamer_push_buffer(void *pipeline, void *buffer, size_t buffer_size, unsigned long duration) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) pipeline;
    if (self == NULL) {
        return;
    }
}

void gstreamer_start_pipeline(void *state) {
    g_print("start");
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) state;
    GstElement *pipeline = self->pipeline;
    gst_element_set_state(pipeline, GST_STATE_PLAYING);

}

void gstreamer_start_main_loop(void* state) {
    loop = g_main_loop_new(NULL, FALSE);
    g_main_loop_run(loop);
}

void gstreamer_stop_main_loop() {
    if (loop != NULL) {
        g_main_loop_quit(loop);
        g_main_loop_unref(loop);
    }
}

void gstreamer_stop_pipeline(void *state) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) state;
    GstElement *pipeline = self->pipeline;
    gst_element_set_state(pipeline, GST_STATE_NULL);
}

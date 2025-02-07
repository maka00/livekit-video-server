#include "gstreamer_wrapper.h"
#include <gst/gst.h>

#include <gst/app/gstappsrc.h>
#include <gst/app/gstappsink.h>

void gstreamer_on_new_sample(GstAppSink *appsink, void* user_data);
void gstreamer_on_new_sample_high(GstAppSink *appsink, void* user_data);
void gstreamer_on_new_sample_medium(GstAppSink *appsink, void* user_data);
void gstreamer_on_new_sample_low(GstAppSink *appsink, void* user_data);
void gstreamer_on_new_sample_quality(GstAppSink *appsink, void* user_data, int quality);
void gstreamer_on_need_data(GstAppSrc *appsrc, guint unused_size, void* user_data);

gboolean gstreamer_bus_watch(GstBus *bus, GstMessage *msg, void *user_data);
static GMainLoop *loop = NULL;

typedef struct {
    GstElement *pipeline;
    GstElement *source_element;
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
void gstreamer_on_need_data(GstAppSrc *appsrc, guint unused_size, void* user_data) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) user_data;
    onNeedData(unused_size, self->id);
}

void gstreamer_on_new_sample(GstAppSink *appsink, void* user_data) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) user_data;
    GstSample *sample = gst_app_sink_pull_sample(appsink);
    GstBuffer *buffer = gst_sample_get_buffer(sample);
    GstMapInfo info;
    if (gst_buffer_map(buffer, &info, GST_MAP_READ)) {
        onNewFrame(info.data, info.size, buffer->duration, self->id, 0);
        gst_buffer_unmap(buffer, &info);
    }
    gst_sample_unref(sample);
}

void gstreamer_on_new_sample_quality(GstAppSink *appsink, void* user_data, int quality) {
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) user_data;
    GstSample *sample = gst_app_sink_pull_sample(appsink);
    GstBuffer *buffer = gst_sample_get_buffer(sample);
    GstMapInfo info;
    if (gst_buffer_map(buffer, &info, GST_MAP_READ)) {
        onNewFrame(info.data, info.size, buffer->duration, self->id, quality);
        gst_buffer_unmap(buffer, &info);
    }
    gst_sample_unref(sample);
}

void gstreamer_on_new_sample_high(GstAppSink *appsink, void* user_data) {
    gstreamer_on_new_sample_quality(appsink, user_data, 2);
}

void gstreamer_on_new_sample_medium(GstAppSink *appsink, void* user_data) {
    gstreamer_on_new_sample_quality(appsink, user_data, 1);
}

void gstreamer_on_new_sample_low(GstAppSink *appsink, void* user_data) {
    gstreamer_on_new_sample_quality(appsink, user_data, 0);
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
    self->source_element = NULL;
    GstElement *app_source = gst_bin_get_by_name(GST_BIN(pipeline), "source");
    if (app_source != NULL) {
        self->source_element = app_source;
        g_signal_connect(app_source, "need-data", G_CALLBACK(gstreamer_on_need_data), self);
    }
    GstElement *app_sink = gst_bin_get_by_name(GST_BIN(pipeline), "sink");
    if (app_sink != NULL) {
        g_signal_connect(app_sink, "new-sample", G_CALLBACK(gstreamer_on_new_sample), self);
    } else {
        GstElement *app_sink_high = gst_bin_get_by_name(GST_BIN(pipeline), "sink_h");
        if (app_sink_high != NULL) {
            g_signal_connect(app_sink_high, "new-sample", G_CALLBACK(gstreamer_on_new_sample_high), self);
        }
        GstElement *app_sink_medium= gst_bin_get_by_name(GST_BIN(pipeline), "sink_m");
        if (app_sink_medium != NULL) {
            g_signal_connect(app_sink_medium, "new-sample", G_CALLBACK(gstreamer_on_new_sample_medium), self);
        }
        GstElement *app_sink_low= gst_bin_get_by_name(GST_BIN(pipeline), "sink_l");
        if (app_sink_low != NULL) {
            g_signal_connect(app_sink_low, "new-sample", G_CALLBACK(gstreamer_on_new_sample_low), self);
        }
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

void gstreamer_push_buffer(void *pipeline, void *buffer, int buffer_size, int duration) {
    g_debug("pushing buffer");
    t_gstreamer_wrapper *self = (t_gstreamer_wrapper *) pipeline;
    if (self == NULL) {
        return;
    }
    gpointer p = g_memdup(buffer, buffer_size);
    //GstBuffer *data = gst_buffer_new_wrapped(p, buffer_size);
    GstBuffer *gst_buffer = gst_buffer_new_wrapped(p, buffer_size);
    // GST_BUFFER_TIMESTAMP(gst_buffer) = duration;
    gst_app_src_push_buffer(GST_APP_SRC(self->source_element), gst_buffer);
    //gstreamer_push_buffer(self->source_element, buffer, buffer_size, duration);
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

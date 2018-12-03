package ffmpeg

/*
#include "ffmpeg.h"
int wrap_avcodec_decode_video2(AVCodecContext *ctx, AVFrame *frame, void *data, int size, int *got) {
    struct AVPacket pkt = {.data = data, .size = size};
    return avcodec_decode_video2(ctx, frame, got, &pkt);
}
*/
import "C"
import (
    "unsafe"
    "runtime"
    "fmt"
    "image"
    "reflect"
    "github.com/nareix/joy4/av"
    "github.com/nareix/joy4/codec/h264parser"
)

type VideoEncoder struct {
    ff *ffctx
    Extradata []byte
}

func (self *VideoEncoder) Encode(images []*image.RGBA, fileOut string, width int, height int) (err error) {
    fmt.Printf("**** Encode Video: %d frames, writing to file %s, width %d, height %d\n", len(images), fileOut, width, height)

    C.av_register_all();
    format_type := "mp4"
    c_format_type := C.CString(format_type)
    defer C.free(unsafe.Pointer(c_format_type))
    c_fileOut := C.CString(fileOut)
    defer C.free(unsafe.Pointer(c_fileOut))
    mpeg_fmt := C.av_guess_format(c_format_type, nil, nil)
    var fc *C.AVFormatContext
    C.avformat_alloc_output_context2(&fc, nil, nil, c_fileOut)
    encoder_name := "libx264"
    c_encoder_name := C.CString(encoder_name)
    defer C.free(unsafe.Pointer(c_encoder_name))
    codec := C.avcodec_find_encoder_by_name(c_encoder_name)
    var opt *C.AVDictionary

    C.av_dict_set(&opt, C.CString("preset"), C.CString("slow"), 0)
    C.av_dict_set(&opt, C.CString("crf"), C.CString("20"), 0)
    stream := C.avformat_new_stream(fc, codec)
    c := stream.codec
    c.width = C.int(width)
    c.height = C.int(height)
    c.pix_fmt = C.AV_PIX_FMT_YUV420P
    var tb C.AVRational
    tb.num = 1
    tb.den = 25
    c.time_base = tb

    // Setting up the format, its stream(s),
    // linking with the codec(s) and write the header.
    if (fc.oformat.flags & C.AVFMT_GLOBALHEADER != 0) {
        // Some formats require a global header.
        c.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
    }
    C.avcodec_open2(c, codec, &opt);
    C.av_dict_free(&opt);

    // Once the codec is set up, we need to let the container know
    // which codec are the streams using, in this case the only (video) stream.
    stream.time_base = tb
    C.av_dump_format(fc, 0, c_fileOut, 1)
    C.avio_open(&fc.pb, c_fileOut, C.AVIO_FLAG_WRITE)
    ret := C.avformat_write_header(fc, &opt)
    if(ret < 0) {
        fmt.Printf("Error writng header")
    }
    C.av_dict_free(&opt)

    // Preparing the containers of the frame data:
    // Allocating memory for each RGB frame, which will be lately converted to YUV.
    rgbpic := C.av_frame_alloc()
    rgbpic.format = C.AV_PIX_FMT_RGBA
    rgbpic.width = C.int(width)
    rgbpic.height = C.int(height)
    ret = C.av_frame_get_buffer(rgbpic, 1)
    if(ret < 0) {
        fmt.Printf("Error")
    }

    // Allocating memory for each conversion output YUV frame.
    yuvpic := C.av_frame_alloc()
    yuvpic.format = C.AV_PIX_FMT_YUV420P
    yuvpic.width = C.int(width)
    yuvpic.height = C.int(height)
    ret = C.av_frame_get_buffer(yuvpic, 1)
    if(ret < 0) {
        fmt.Printf("Error")
    }

    var pkt C.AVPacket
    var got_output C.int
    var iframe C.int

    // Preparing to convert my generated RGB images to YUV frames.
    swsCtx := C.sws_getContext(C.int(width), C.int(height), C.AV_PIX_FMT_RGBA, C.int(width), C.int(height),
        C.AV_PIX_FMT_YUV420P, C.SWS_FAST_BILINEAR, nil, nil, nil)

    for _, img := range images {
        /*
            rgb_pixels := img.Pix
            // should be 4 for rgba
            pix_stride := img.Stride

            // adding frame
            total_bytes := len(rgb_pixels)
            data := C.GoBytes(unsafe.Pointer(rgbpic.data[0]), C.int(total_bytes))
            for y := 0; y < height; y++ {
                for x := 0; x < width; x++ {
                    go_linesize := int(rgbpic.linesize[0])
                    // rgbpic->linesize[0] is equal to width.
                    data[y * go_linesize + pix_stride * x + 0] = rgb_pixels[y * 4 * width + 4 * x + 3]
                    data[y * go_linesize + pix_stride * x + 1] = rgb_pixels[y * 4 * width + 4 * x + 2]
                    data[y * go_linesize + pix_stride * x + 2] = rgb_pixels[y * 4 * width + 4 * x + 1]
                    data[y * go_linesize + pix_stride * x + 3] = rgb_pixels[y * 4 * width + 4 * x + 0]
                }
            }
     */
        //C.avio_flush(&fc.pb)
        c_data := rgbpic.data[0]
        //byte_count := width * height * 4
		var byte_count int
		byte_count = int(rgbpic.linesize[0] * rgbpic.height)
        var go_data []byte
        sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&go_data)))
        sliceHeader.Cap = byte_count
        sliceHeader.Len = byte_count
        sliceHeader.Data = uintptr(unsafe.Pointer(c_data))

        //data := C.GoBytes(unsafe.Pointer(rgbpic.data[0]), C.int(byte_count))
		go_linesize := int(rgbpic.linesize[0])
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				// R
				go_data[y * go_linesize + 4 * x + 0] = img.Pix[(y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4 + 0]
				// G
				go_data[y * go_linesize + 4 * x + 1] = img.Pix[(y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4 + 1]
				// B
				go_data[y * go_linesize + 4 * x + 2] = img.Pix[(y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4 + 2]
				// A
				go_data[y * go_linesize + 4 * x + 3] = img.Pix[(y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4 + 3]
			}
		}
		//for i := 0; i < byte_count; i++ {
        //    go_data[i] = img.Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4]
        //}

        // Not actually scaling anything, but just converting
        // the RGB data to YUV and store it in yuvpic.
        C.sws_scale(swsCtx, (**C.uchar)(unsafe.Pointer(&rgbpic.data)), (*C.int)(unsafe.Pointer(&rgbpic.linesize)), 0, C.int(height),
            (**C.uchar)(unsafe.Pointer(&yuvpic.data)), (*C.int)(unsafe.Pointer(&yuvpic.linesize)))


        C.av_init_packet(&pkt)
        pkt.data = nil
        pkt.size = 0

        // The PTS of the frame are just in a reference unit,
        // unrelated to the format we are using. We set them,
        // for instance, as the corresponding frame number.
        yuvpic.pts = C.long(iframe)

        ret = C.avcodec_encode_video2(c, &pkt, yuvpic, &got_output)

        if (ret != 0) {
            fmt.Println("Error encoding frame into packet: %d", ret)
            continue
        }
		iframe += 1
        // what are we counting here
        if (got_output != 0) {
            // We set the packet PTS and DTS taking in the account our FPS (second argument),
            // and the time base that our selected format uses (third argument).
            C.av_packet_rescale_ts(&pkt, tb, stream.time_base)

            pkt.stream_index = stream.index
            fmt.Printf("Writing frame %d (size = %d)\n", iframe, pkt.size)
            //fmt.Printf("Frame %d, seq num %d\n", i, img.frame.coded_picture_number)

            // Write the encoded frame to the mp4 file.
            C.av_interleaved_write_frame(fc, &pkt)
            //C.av_write_frame(fc, &pkt)
            C.av_packet_unref(&pkt)
        }
    }

    // flush file
    // Writing the delayed frames:
    for true {
        //C.avio_flush(&fc.pb)
        ret = C.avcodec_encode_video2(c, &pkt, nil, &got_output)
        if got_output == 1 {
            C.av_packet_rescale_ts(&pkt, tb, stream.time_base)
            pkt.stream_index = stream.index
            fmt.Printf("Writing delayed frame %d (size = %d)\n", iframe, pkt.size)
            //iframe += 1
            C.av_interleaved_write_frame(fc, &pkt)
            C.av_packet_unref(&pkt)
        }else{break}
    }

    // Writing the end of the file.
    C.av_write_trailer(fc)

    // Closing the file.
    if (mpeg_fmt.flags & C.AVFMT_NOFILE == 0) {
        C.avio_closep(&fc.pb)
    }
    C.avcodec_close(stream.codec)

    // Freeing all the allocated memory:
    C.av_frame_free(&rgbpic)
    C.av_frame_free(&yuvpic)
    C.avformat_free_context(fc)

    return
}

type VideoDecoder struct {
    ff *ffctx
    Extradata []byte
}

func (self *VideoDecoder) Setup() (err error) {
    ff := &self.ff.ff
    if len(self.Extradata) > 0 {
        ff.codecCtx.extradata = (*C.uint8_t)(unsafe.Pointer(&self.Extradata[0]))
        ff.codecCtx.extradata_size = C.int(len(self.Extradata))
    }
    if C.avcodec_open2(ff.codecCtx, ff.codec, nil) != 0 {
        err = fmt.Errorf("ffmpeg: decoder: avcodec_open2 failed")
        return
    }
    return
}

func fromCPtr(buf unsafe.Pointer, size int) (ret []uint8) {
    hdr := (*reflect.SliceHeader)((unsafe.Pointer(&ret)))
    hdr.Cap = size
    hdr.Len = size
    hdr.Data = uintptr(buf)
    return
}

type VideoFrame struct {
    Image image.YCbCr
    RGBA *image.RGBA
    frame *C.AVFrame
}

func (self *VideoFrame) Free() {
    self.Image = image.YCbCr{}
    C.av_frame_free(&self.frame)
}

func freeVideoFrame(self *VideoFrame) {
    self.Free()
}

func (self *VideoDecoder) Decode(pkt []byte) (img *VideoFrame, err error) {
    ff := &self.ff.ff

    cgotimg := C.int(0)
    frame := C.av_frame_alloc()
    cerr := C.wrap_avcodec_decode_video2(ff.codecCtx, frame, unsafe.Pointer(&pkt[0]), C.int(len(pkt)), &cgotimg)
    if cerr < C.int(0) {
        err = fmt.Errorf("ffmpeg: avcodec_decode_video2 failed: %d", cerr)
        return
    }

    if cgotimg != C.int(0) {
        w := int(frame.width)
        h := int(frame.height)
        ys := int(frame.linesize[0])
        cs := int(frame.linesize[1])

        img = &VideoFrame{Image: image.YCbCr{
            Y: fromCPtr(unsafe.Pointer(frame.data[0]), ys*h),
            Cb: fromCPtr(unsafe.Pointer(frame.data[1]), cs*h/2),
            Cr: fromCPtr(unsafe.Pointer(frame.data[2]), cs*h/2),
            YStride: ys,
            CStride: cs,
            SubsampleRatio: image.YCbCrSubsampleRatio420,
            Rect: image.Rect(0, 0, w, h),
        }, frame: frame}
        runtime.SetFinalizer(img, freeVideoFrame)
    }

    return
}

func NewVideoDecoder(stream av.CodecData) (dec *VideoDecoder, err error) {
    _dec := &VideoDecoder{}
    var id uint32

    switch stream.Type() {
    case av.H264:
        h264 := stream.(h264parser.CodecData)
        _dec.Extradata = h264.AVCDecoderConfRecordBytes()
        id = C.AV_CODEC_ID_H264

    default:
        err = fmt.Errorf("ffmpeg: NewVideoDecoder codec=%v unsupported", stream.Type())
        return
    }

    c := C.avcodec_find_decoder(id)
    if c == nil || C.avcodec_get_type(id) != C.AVMEDIA_TYPE_VIDEO {
        err = fmt.Errorf("ffmpeg: cannot find video decoder codecId=%d", id)
        return
    }

    if _dec.ff, err = newFFCtxByCodec(c); err != nil {
        return
    }
    if err =  _dec.Setup(); err != nil {
        return
    }

    dec = _dec
    return
}

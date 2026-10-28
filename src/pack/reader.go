package pack

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Reader provides bounded streaming reads directly from the SFX file.
type Reader struct {
	f *os.File
	entries map[string]Entry
	ordered []Entry
	digest [32]byte
}
// DigestHex is a stable cache key for one immutable SFX index.
func (r *Reader)DigestHex() string { return hex.EncodeToString(r.digest[:]) }
func (r *Reader)Close()error{return r.f.Close()}
func (r *Reader)Files()[]Entry{return append([]Entry(nil),r.ordered...)}
// Open implements domain.PayloadSource without pre-extracting installers.
func (r *Reader)Open(rel string)(io.ReadCloser,error){
	if err:=safePath(rel);err!=nil{return nil,err}
	e,ok:=r.entries[rel]
	if !ok{return nil,fmt.Errorf("E-SFX-003: arquivo não encontrado: %s",rel)}
	return io.NopCloser(io.NewSectionReader(r.f,e.Offset,e.Size)),nil
}
func (r *Reader)Verify(rel string)error{
	f,err:=r.Open(rel);if err!=nil{return err}
	defer f.Close()
	e:=r.entries[rel]
	h:=sha256.New()
	n,err:=io.CopyBuffer(h,f,make([]byte,1<<20))
	if err!=nil{return err}
	if n!=e.Size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)),e.SHA256){
		return fmt.Errorf("E-SFX-002: integridade incorreta de %s",rel)
	}
	return nil
}
func (r *Reader)VerifyAll()error{
	for _,e:=range r.ordered{if err:=r.Verify(e.Path);err!=nil{return err}}
	return nil
}
func readExactly(ra io.ReaderAt,offset int64,buf []byte)error{
	_,err:=ra.ReadAt(buf,offset)
	return err
}
func archiveEnd(f *os.File,size int64)(int64,error){
	executable,err:=pe.NewFile(f)
	if err!=nil{
		// Allow pure-data test fixtures to use the same parser. A Windows
		// running EXE is expected to be a valid PE file.
		return size,nil
	}
	defer executable.Close()
	var start,certSize uint32
	switch h:=executable.OptionalHeader.(type){
	case *pe.OptionalHeader64:
		start=h.DataDirectory[4].VirtualAddress
		certSize=h.DataDirectory[4].Size
	case *pe.OptionalHeader32:
		start=h.DataDirectory[4].VirtualAddress
		certSize=h.DataDirectory[4].Size
	default:
		return 0,errors.New("E-SFX-001: cabeçalho PE inválido")
	}
	if start==0 && certSize==0{return size,nil}
	if start==0 || certSize==0 || int64(start)>size || int64(certSize)>size-int64(start){
		return 0,errors.New("E-SFX-001: tabela de certificado fora do executável")
	}
	// The PE security directory uses FILE offsets, not virtual addresses.
	// The package footer precedes alignment padding and Authenticode.
	if start%8!=0{return 0,errors.New("E-SFX-001: certificado PE desalinhado")}
	return int64(start),nil
}
// OpenFile detects the index footer before an optional Authenticode certificate.
// It rejects malformed, overlapping, out-of-range or duplicated entries.
func OpenFile(filename string)(*Reader,error){
	f,err:=os.Open(filename)
	if err!=nil{return nil,err}
	reader,err:=openReader(f)
	if err!=nil{f.Close();return nil,err}
	return reader,nil
}
func openReader(f *os.File)(*Reader,error){
	stat,err:=f.Stat();if err!=nil{return nil,err}
	end,err:=archiveEnd(f,stat.Size());if err!=nil{return nil,err}
	var indexStart,indexLength int64
	var expected [32]byte
	var footerAt int64=-1
	for pad:=int64(0);pad<8 && end-pad>=footerSize;pad++{
		if pad>0 {
			extra:=make([]byte,pad)
			if err:=readExactly(f,end-pad,extra);err!=nil{return nil,err}
			if !bytes.Equal(extra,make([]byte,pad)){continue}
		}
		buf:=make([]byte,footerSize)
		start:=end-pad-footerSize
		if err:=readExactly(f,start,buf);err!=nil{return nil,err}
		if string(buf[:8])!=magic{continue}
		offset:=binary.LittleEndian.Uint64(buf[8:16])
		length:=binary.LittleEndian.Uint64(buf[16:24])
		if offset>uint64(start) || length==0 || length>maxIndexBytes || offset+length!=uint64(start){
			return nil,errors.New("E-SFX-001: posições inválidas do índice")
		}
		indexStart=int64(offset)
		indexLength=int64(length)
		copy(expected[:],buf[24:])
		footerAt=start
		break
	}
	if footerAt<0{return nil,errors.New("E-SFX-001: rodapé SFX ausente")}
	raw:=make([]byte,indexLength)
	if err:=readExactly(f,indexStart,raw);err!=nil{return nil,err}
	if sha256.Sum256(raw)!=expected{return nil,errors.New("E-SFX-002: índice SFX alterado")}
	dec:=json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var index Index
	if err:=dec.Decode(&index);err!=nil{return nil,fmt.Errorf("E-SFX-001: índice inválido: %w",err)}
	var trailing any
	if err:=dec.Decode(&trailing);err!=io.EOF{return nil,errors.New("E-SFX-001: dados extras no índice")}
	if index.Schema!=1 || len(index.Files)==0{return nil,errors.New("E-SFX-001: índice vazio ou versão inválida")}
	entries:=map[string]Entry{}
	ordered:=append([]Entry(nil),index.Files...)
	sort.Slice(ordered,func(i,j int)bool{return ordered[i].Offset<ordered[j].Offset})
	var previous int64=-1
	folded:=map[string]bool{}
	for _,entry:=range ordered{
		if err:=safePath(entry.Path);err!=nil{return nil,err}
		key:=strings.ToLower(entry.Path)
		if folded[key]{return nil,fmt.Errorf("E-SFX-001: arquivo repetido %s",entry.Path)}
		folded[key]=true
		if entry.Offset<0 || entry.Size<=0 || entry.Offset>indexStart || entry.Size>indexStart-entry.Offset ||
			(previous>=0 && entry.Offset<previous){
			return nil,fmt.Errorf("E-SFX-001: offset/size inválidos para %s",entry.Path)
		}
		if len(entry.SHA256)!=64 {
			return nil,fmt.Errorf("E-SFX-001: SHA256 incorreto para %s",entry.Path)
		}
		if _,err:=hex.DecodeString(entry.SHA256);err!=nil{
			return nil,fmt.Errorf("E-SFX-001: SHA256 inválido para %s",entry.Path)
		}
		previous=entry.Offset+entry.Size
		entries[entry.Path]=entry
	}
	return &Reader{f:f,entries:entries,ordered:ordered,digest:expected},nil
}

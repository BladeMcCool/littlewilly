package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/roasbeef/btcutil"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	port                   = ":8086"
	lnRpcHost              = "127.0.0.1:10002"
	defaultTLSCertFilename = "tls.cert"
)

var (
	calls              = 0
	lndHomeDir         = btcutil.AppDataDir("lnd", false)
	defaultTLSCertPath = filepath.Join(lndHomeDir, defaultTLSCertFilename)
)

func getLNRpc() (lnrpc.LightningClient, func() error) {
	rpcClient := getRPCClientConn()
	lnRPC := lnrpc.NewLightningClient(rpcClient)
	return lnRPC, rpcClient.Close
}

func getRPCClientConn() *grpc.ClientConn {
	// Load the specified TLS certificate and build transport credentials
	// with it.
	tlsCertPath := cleanAndExpandPath(defaultTLSCertPath)
	creds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fatal(err)
	}

	// Create a dial options array.
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	conn, err := grpc.Dial(lnRpcHost, opts...)
	if err != nil {
		fatal(err)
	}

	return conn
}

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	calls++

	ctxb := context.Background()

	fmt.Printf("lets invoice\n")
	lnRpc, cleanup := getLNRpc()
	defer cleanup()

	// req := &lnrpc.ListChannelsRequest{}
	// printResp, err := lnRpc.ListChannels(ctxb, req)
	// theJson := getJsonPbStr(printResp)

	invoice := &lnrpc.Invoice{
		Value: int64(1234),
	}

	resp, err := lnRpc.AddInvoice(ctxb, invoice)
	if err != nil {
		fmt.Fprint(w, err.Error())
		return
	}

	printResp := struct {
		R string
		P string
	}{
		R: fmt.Sprintf("%x", resp.RHash),
		P: resp.PaymentRequest,
	}
	theJson := getJsonStr(printResp)

	fmt.Fprint(w, theJson)
}

func Check(w http.ResponseWriter, r *http.Request) {
	lnRpc, cleanup := getLNRpc()
	defer cleanup()
	fmt.Printf("lets check\n")
	rHash, err := hex.DecodeString(r.URL.Query().Get("r"))
	if err != nil {
		nonFatal(err, w)
		return
	}

	req := &lnrpc.PaymentHash{
		RHash: rHash,
	}

	invoice, err := lnRpc.LookupInvoice(context.Background(), req)
	if err != nil {
		nonFatal(err, w)
		return
	}

	printResp := struct {
		Settled bool
	}{
		Settled: invoice.Settled,
	}
	theJson := getJsonStr(printResp)

	// theJson := getJsonPbStr(invoice)
	fmt.Fprint(w, theJson)
}

func NoOp(w http.ResponseWriter, r *http.Request) {
	//crickets.
}

func init() {
	fmt.Printf("Started server at http://localhost%v.\n", port)
	http.HandleFunc("/", HelloWorld)
	http.HandleFunc("/favicon.ico", NoOp)
	http.HandleFunc("/check/", Check)
	http.ListenAndServe(port, nil)
}

func main() {}

func getJsonPbStr(resp proto.Message) string {
	jsonMarshaler := &jsonpb.Marshaler{
		EmitDefaults: true,
		Indent:       "    ",
	}

	jsonStr, err := jsonMarshaler.MarshalToString(resp)
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return ""
	}
	return jsonStr
}
func getJsonStr(resp interface{}) string {
	b, err := json.Marshal(resp)
	if err != nil {
		fatal(err)
	}
	return string(b)
}

// func printRespJSON(resp proto.Message) {
// 	fmt.Println(getJsonStr(resp))
// }
func nonFatal(err error, w io.Writer) {
	fmt.Fprint(w, err.Error())
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[willllllliaaam] %v\n", err)
	os.Exit(1)
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(lndHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

// Server represents the API server
type Server struct {
	store  store.Store
	router *mux.Router
	port   int
}

// NewServer creates a new API server
func NewServer(store store.Store, port int) *Server {
	s := &Server{
		store:  store,
		router: mux.NewRouter(),
		port:   port,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/healthz", s.healthHandler).Methods("GET")
	s.router.HandleFunc("/readyz", s.readyHandler).Methods("GET")

	// API v1alpha1
	apiV1 := s.router.PathPrefix("/api/v1alpha1").Subrouter()

	// Pods
	apiV1.HandleFunc("/namespaces/{namespace}/pods", s.createPod).Methods("POST")
	apiV1.HandleFunc("/namespaces/{namespace}/pods", s.listPods).Methods("GET")
	apiV1.HandleFunc("/namespaces/{namespace}/pods/{name}", s.getPod).Methods("GET")
	apiV1.HandleFunc("/namespaces/{namespace}/pods/{name}", s.updatePod).Methods("PUT")
	apiV1.HandleFunc("/namespaces/{namespace}/pods/{name}", s.deletePod).Methods("DELETE")
	apiV1.HandleFunc("/namespaces/{namespace}/pods/{name}/watch", s.watchPod).Methods("GET")

	// Nodes
	apiV1.HandleFunc("/nodes", s.createNode).Methods("POST")
	apiV1.HandleFunc("/nodes", s.listNodes).Methods("GET")
	apiV1.HandleFunc("/nodes/{name}", s.getNode).Methods("GET")
	apiV1.HandleFunc("/nodes/{name}", s.updateNode).Methods("PUT")
	apiV1.HandleFunc("/nodes/{name}", s.deleteNode).Methods("DELETE")
	apiV1.HandleFunc("/nodes/{name}/watch", s.watchNode).Methods("GET")

	// All pods (for listing across namespaces)
	apiV1.HandleFunc("/pods", s.listAllPods).Methods("GET")
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting API server on %s\n", addr)
	return http.ListenAndServe(addr, s.router)
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// readyHandler handles readiness check requests
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// createPod handles pod creation
func (s *Server) createPod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	var pod api.Pod
	if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set metadata
	pod.Kind = "Pod"
	pod.APIVersion = "v1alpha1"
	pod.Namespace = namespace
	pod.UID = generateUID()
	pod.Status.Phase = string(api.PodPending)

	// Create in store
	ctx := r.Context()
	if err := s.store.Create(ctx, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pod)
}

// getPod handles pod retrieval
func (s *Server) getPod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	ctx := r.Context()
	pod, err := s.store.Get(ctx, "Pod", namespace, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pod)
}

// listPods handles pod listing
func (s *Server) listPods(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	ctx := r.Context()
	pods, err := s.store.List(ctx, "Pod", namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to proper pod slice
	var podList []*api.Pod
	for _, obj := range pods {
		if pod, ok := obj.(*api.Pod); ok {
			podList = append(podList, pod)
		}
	}

	response := map[string]interface{}{
		"apiVersion": "v1alpha1",
		"kind":       "PodList",
		"items":      podList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// listAllPods handles listing pods across all namespaces
func (s *Server) listAllPods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// For now, just list from default namespace
	// In a real implementation, you'd want to aggregate across namespaces
	pods, err := s.store.List(ctx, "Pod", "default")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var podList []*api.Pod
	for _, obj := range pods {
		if pod, ok := obj.(*api.Pod); ok {
			podList = append(podList, pod)
		}
	}

	response := map[string]interface{}{
		"apiVersion": "v1alpha1",
		"kind":       "PodList",
		"items":      podList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updatePod handles pod updates
func (s *Server) updatePod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	var pod api.Pod
	if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set metadata
	pod.Kind = "Pod"
	pod.APIVersion = "v1alpha1"
	pod.Namespace = namespace
	pod.Name = name

	ctx := r.Context()
	if err := s.store.Update(ctx, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pod)
}

// deletePod handles pod deletion
func (s *Server) deletePod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	ctx := r.Context()
	if err := s.store.Delete(ctx, "Pod", namespace, name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// watchPod handles pod watch requests
func (s *Server) watchPod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	ctx := r.Context()
	watchResult, err := s.store.Watch(ctx, "Pod", namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for streaming
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// Flush headers
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Stream events
	for {
		select {
		case event := <-watchResult.Events:
			// Filter events for the specific pod
			if pod, ok := event.Object.(*api.Pod); ok && pod.Name == name {
				eventJSON, _ := json.Marshal(event)
				w.Write(eventJSON)
				w.Write([]byte("\n"))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		case <-watchResult.Stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// createNode handles node creation
func (s *Server) createNode(w http.ResponseWriter, r *http.Request) {
	var node api.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set metadata
	node.Kind = "Node"
	node.APIVersion = "v1alpha1"
	node.UID = generateUID()

	ctx := r.Context()
	if err := s.store.Create(ctx, &node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// getNode handles node retrieval
func (s *Server) getNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	ctx := r.Context()
	node, err := s.store.Get(ctx, "Node", "", name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// listNodes handles node listing
func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := s.store.List(ctx, "Node", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var nodeList []*api.Node
	for _, obj := range nodes {
		if node, ok := obj.(*api.Node); ok {
			nodeList = append(nodeList, node)
		}
	}

	response := map[string]interface{}{
		"apiVersion": "v1alpha1",
		"kind":       "NodeList",
		"items":      nodeList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateNode handles node updates
func (s *Server) updateNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var node api.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set metadata
	node.Kind = "Node"
	node.APIVersion = "v1alpha1"
	node.Name = name

	ctx := r.Context()
	if err := s.store.Update(ctx, &node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// deleteNode handles node deletion
func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	ctx := r.Context()
	if err := s.store.Delete(ctx, "Node", "", name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// watchNode handles node watch requests
func (s *Server) watchNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	ctx := r.Context()
	watchResult, err := s.store.Watch(ctx, "Node", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for streaming
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// Flush headers
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Stream events
	for {
		select {
		case event := <-watchResult.Events:
			// Filter events for the specific node
			if node, ok := event.Object.(*api.Node); ok && node.Name == name {
				eventJSON, _ := json.Marshal(event)
				w.Write(eventJSON)
				w.Write([]byte("\n"))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		case <-watchResult.Stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// generateUID generates a unique identifier
func generateUID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

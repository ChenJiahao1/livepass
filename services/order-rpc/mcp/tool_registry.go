package ordermcp

func registerTools(server *Server) {
	registerOrderTools(server)
	registerRefundTools(server)
}

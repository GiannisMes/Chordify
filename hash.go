package main

import (
	"crypto/sha1" //απο  εδω παιρνω την hash function που ζηταει η εκφωνηση
	"math/big"
)

func hashString(elt string) *big.Int {
	hasher := sha1.New()      //υπολογισμος hash
	hasher.Write([]byte(elt)) // κανει το κειμενο σε  bytes
	hashBytes := hasher.Sum(nil)

	result := new(big.Int)
	result.SetBytes(hashBytes)
	return result
}

/*ουσιαστικα αυτη το η συναρτηση μετατρεπει το ip port kαθε κoμβου σε node id , και παιρνει το ονομα
ενος τραγουδιου και το μετατρεπει σε ενα key id */

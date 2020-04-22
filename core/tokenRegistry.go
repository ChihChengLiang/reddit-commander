package types

type Token struct {
	DBModel
	// Token Address on eth chain.
	Address Address `json:"address"`

	// token ID allocated to the token
	TokenID uint64 `json:"tokenID"`
}

func (db *DB) AddToken(t Token) error {
	return db.Instance.Create(t).Error
}

func (db *DB) GetTokenByID(id uint) (token types.Token, err error) {
	err = db.Instance.Where("token_id = ?", id).First(&token).Error
	if err != nil {
		return
	}
	return
}

package upgrademanager

import (
	"encoding/hex"

	"github.com/lbryio/chainquery/daemon/processing"
	"github.com/lbryio/chainquery/lbrycrd"
	"github.com/lbryio/chainquery/model"
	"github.com/lbryio/chainquery/util"

	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func reProcessAllClaims() {
	outputs := model.OutputsG(qm.Where(model.OutputColumns.Type+" =?", lbrycrd.NonStandard),
		qm.Select(model.OutputColumns.TransactionHash)).AllP()
	for i, output := range outputs {
		processClaimOut(i, len(outputs), output.TransactionHash)
	}
}

func processClaimOut(index int, total int, txHash string) {
	tx, err := model.TransactionsG(qm.Where(model.TransactionColumns.Hash+"=?", txHash),
		qm.Select(model.TransactionColumns.Hash, model.TransactionColumns.BlockByHashID)).One()
	if err != nil {
		logrus.Panic(err)
	}
	txResult, err := lbrycrd.GetRawTransactionResponse(tx.Hash)
	if err != nil {
		logrus.Panic(err)
	}

	block, err := model.BlocksG(qm.Where(model.BlockColumns.Hash+"=?", txResult.BlockHash)).One()
	if err != nil {
		logrus.Panic(err)
	}
	if index%50 == 0 {
		logrus.Info("(", index, "/", total, ")", "Processing@Height ", block.Height)
	}

	err = processing.ProcessTx(txResult, block.BlockTime, block.Height)
	if err != nil {
		logrus.Error("Reprocess Claim Error: ", err)
	}
}

func setClaimAddresses() {
	type claimForClaimAddress struct {
		ID              uint64 `boil:"id"`
		ScriptPubKeyHex string `boil:"script_pub_key_hex"`
		ClaimAddress    string `boil:"claim_address"`
	}
	rows, err := boil.GetDB().Query(`
		SELECT claim.id,output.script_pub_key_hex FROM claim
		LEFT JOIN output ON output.transaction_hash = claim.transaction_by_hash_id AND output.vout = claim.vout
		WHERE claim_address = ''`)

	if err != nil {
		logrus.Panic("Error During Upgrade: ", err)
	}
	defer util.CloseRows(rows)

	var slice []claimForClaimAddress
	for rows.Next() {
		var claimForCA claimForClaimAddress
		err = rows.Scan(&claimForCA.ID, &claimForCA.ScriptPubKeyHex)
		if err != nil {
			logrus.Panic("Error During Upgrade: ", err)
		}
		slice = append(slice, claimForCA)
	}

	for i, claimAddress := range slice {
		if i%1000 == 0 {
			logrus.Info("Processing: ", "(", i, "/", len(slice), ")")
		}
		claim := model.Claim{ID: claimAddress.ID}
		scriptBytes, err := hex.DecodeString(claimAddress.ScriptPubKeyHex)
		if err != nil {
			logrus.Panic("Error During Upgrade: ", err)
		}
		var pkscript []byte
		if lbrycrd.IsClaimScript(scriptBytes) {
			if lbrycrd.IsClaimNameScript(scriptBytes) {
				_, _, pkscript, err = lbrycrd.ParseClaimNameScript(scriptBytes)
			} else if lbrycrd.IsClaimUpdateScript(scriptBytes) {
				_, _, _, pkscript, err = lbrycrd.ParseClaimUpdateScript(scriptBytes)
			} else if lbrycrd.IsClaimSupportScript(scriptBytes) {
				_, _, pkscript, err = lbrycrd.ParseClaimSupportScript(scriptBytes)
			} else {
				continue
			}
			if err != nil {
				logrus.Error("Error Parsing claim script: ", err)
				continue
			}
			pksAddress := lbrycrd.GetAddressFromPublicKeyScript(pkscript)
			claim.ClaimAddress = pksAddress
			if err := claim.UpdateG(model.ClaimColumns.ClaimAddress); err != nil {
				logrus.Error("Saving Claim Address Error: ", err)
			}
		}
	}
}

func setBlockHeightOnAllClaims() {

	type claimInfo struct {
		ID       uint64 `boil:"id"`
		claim_id string `boil:"claim_id"`
		height   uint64 `boil:"height"`
	}

	rows, err := boil.GetDB().Query(`
	SELECT c.id,b.height
	FROM claim c
	LEFT JOIN transaction t on c.transaction_by_hash_id = t.hash
	LEFT JOIN block b on b.hash = t.block_by_hash_id
	WHERE c.height = 0`)

	if err != nil {
		logrus.Panic("Error During Upgrade: ", err)
	}
	defer util.CloseRows(rows)

	var slice []claimInfo

	for rows.Next() {
		var info claimInfo
		err = rows.Scan(&info.ID, &info.height)
		if err != nil {
			logrus.Panic("Error During Upgrade: ", err)
		}
		slice = append(slice, info)
	}

	for i, info := range slice {
		if i%1000 == 0 {
			logrus.Info("Processing: ", "(", i, "/", len(slice), ")")
		}

		claim := model.Claim{ID: info.ID, Height: uint(info.height)}
		if err := claim.UpdateG(model.ClaimColumns.Height); err != nil {
			println(err)
		}
	}
}

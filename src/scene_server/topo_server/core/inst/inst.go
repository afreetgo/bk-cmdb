/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package inst

import (
	"context"
	"encoding/json"

	"configcenter/src/apimachinery"
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/condition"
	frtypes "configcenter/src/common/mapstr"
	metatype "configcenter/src/common/metadata"
	"configcenter/src/scene_server/topo_server/core/model"
	"configcenter/src/scene_server/topo_server/core/types"
)

type inst struct {
	clientSet apimachinery.ClientSetInterface
	params    types.LogicParams
	datas     frtypes.MapStr
	target    model.Object
}

func (cli *inst) MarshalJSON() ([]byte, error) {
	return json.Marshal(cli.datas)
}

func (cli *inst) searchInsts(targetModel model.Object, cond condition.Condition) ([]Inst, error) {

	queryInput := &metatype.QueryInput{}
	queryInput.Condition = cond.ToMapStr()

	rsp, err := cli.clientSet.ObjectController().Instance().SearchObjects(context.Background(), targetModel.GetObjectType(), cli.params.Header.ToHeader(), queryInput)
	if nil != err {
		blog.Errorf("[inst-inst] failed to request the object controller , error info is %s", err.Error())
		return nil, cli.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("[inst-inst] failed to search the inst, error info is %s", rsp.ErrMsg)
		return nil, cli.params.Err.Error(rsp.Code)
	}

	blog.Infof("debug inst:%#v", rsp.Data.Info)

	return CreateInst(cli.params, cli.clientSet, targetModel, rsp.Data.Info), nil

}

func (cli *inst) Create() error {

	if cli.target.IsCommon() {
		cli.datas.Set(common.BKObjIDField, cli.target.GetID())
	}

	rsp, err := cli.clientSet.ObjectController().Instance().CreateObject(context.Background(), cli.target.GetObjectType(), cli.params.Header.ToHeader(), cli.datas)
	if nil != err {
		blog.Errorf("failed to create object instance, error info is %s", err.Error())
		return err
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to create object instance ,error info is %v", rsp.ErrMsg)
		return cli.params.Err.Error(common.CCErrTopoInstCreateFailed)
	}

	id, exists := rsp.Data.Get(cli.target.GetInstIDFieldName())
	if !exists {
		blog.Warnf("the object controller return the creatation result is invalid, lost the inst id (%s) int the result data(%#v)", cli.target.GetInstIDFieldName(), rsp.Data)
	}

	cli.datas.Set(cli.target.GetInstIDFieldName(), id)

	return nil
}

func (cli *inst) Update() error {

	instIDName := cli.target.GetInstIDFieldName()
	instID, exists := cli.datas.Get(instIDName)

	cond := condition.CreateCondition()

	if cli.target.IsCommon() {
		cond.Field(common.BKObjIDField).Eq(cli.target.GetID())
	}

	if exists {
		// construct the update condition by the instid
		cond.Field(instIDName).Eq(instID)
	} else {
		// construct the update condition by the only key

		attrs, err := cli.target.GetAttributes()
		if nil != err {
			blog.Errorf("failed to get attributes for the object(%s), error info is is %s", cli.target.GetID(), err.Error())
			return err
		}

		for _, attrItem := range attrs {
			// check the inst
			if attrItem.GetIsOnly() {

				val, exists := cli.datas.Get(attrItem.GetID())
				if !exists {
					continue
				}
				cond.Field(attrItem.GetID()).Eq(val)
			}
		}

	}

	// execute update action
	updateCond := frtypes.MapStr{}
	updateCond.Set("data", cli.datas)
	updateCond.Set("condition", cond.ToMapStr())
	rsp, err := cli.clientSet.ObjectController().Instance().UpdateObject(context.Background(), cli.target.GetObjectType(), cli.params.Header.ToHeader(), updateCond)
	if nil != err {
		blog.Errorf("failed to update the object(%s) instances, error info is %s", cli.target.GetID(), err.Error())
		return cli.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to update the object(%s) instances, error info is %s", cli.target.GetID(), rsp.ErrMsg)
		return cli.params.Err.Error(common.CCErrTopoInstUpdateFailed)
	}

	// read the new data
	instItems, err := cli.searchInsts(cli.target, cond)
	if nil != err {
		blog.Errorf("[inst-inst] failed to search the new insts data, error info is %s", err.Error())
		return err
	}

	for _, item := range instItems { // should be only one item
		cli.datas = item.GetValues()
	}

	return nil
}
func (cli *inst) Delete() error {

	instIDName := cli.target.GetInstIDFieldName()
	instID, exists := cli.datas.Get(instIDName)

	cond := condition.CreateCondition()

	if exists {
		// construct the delete condition by the instid
		cond.Field(instIDName).Eq(instID)
	} else {
		// construct the delete condition by the only key

		attrs, err := cli.target.GetAttributes()
		if nil != err {
			blog.Errorf("failed to get attributes for the object(%s), error info is is %s", cli.target.GetID(), err.Error())
			return err
		}

		for _, attrItem := range attrs {
			// check the inst
			if attrItem.GetIsOnly() {

				val, exists := cli.datas.Get(attrItem.GetID())
				if !exists {
					continue
				}
				cond.Field(attrItem.GetID()).Eq(val)
			}
		}

	}

	// execute delete action
	rsp, err := cli.clientSet.ObjectController().Instance().DelObject(context.Background(), cli.target.GetObjectType(), cli.params.Header.ToHeader(), cond.ToMapStr())
	if nil != err {
		blog.Errorf("failed to delete the object(%s) instances, error info is %s", cli.target.GetID(), err.Error())
		return cli.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to delete the object(%s) instances, error info is %s", cli.target.GetID(), rsp.ErrMsg)
		return cli.params.Err.Error(common.CCErrTopoInstUpdateFailed)
	}

	return nil
}
func (cli *inst) IsExists() (bool, error) {

	attrs, err := cli.target.GetAttributes()
	if nil != err {
		blog.Errorf("failed to get attributes for the object(%s), error info is is %s", cli.target.GetID(), err.Error())
		return false, err
	}

	cond := condition.CreateCondition()
	if cli.target.IsCommon() {
		cond.Field(common.BKObjIDField).Eq(cli.target.GetID())
	}

	for _, attrItem := range attrs {
		// check the inst
		if attrItem.GetIsOnly() {

			val, exists := cli.datas.Get(attrItem.GetID())
			if !exists {
				return false, cli.params.Err.Errorf(common.CCErrCommParamsLostField, attrItem.GetID())
			}
			cond.Field(attrItem.GetID()).Eq(val)
		}
	}

	queryCond := metatype.QueryInput{}
	queryCond.Condition = cond.ToMapStr()

	rsp, err := cli.clientSet.ObjectController().Instance().SearchObjects(context.Background(), cli.target.GetObjectType(), cli.params.Header.ToHeader(), &queryCond)

	if nil != err {
		blog.Errorf("failed to search object(%s) instances  , error info is %s", cli.target.GetID(), err.Error())
		return false, cli.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to search the object (%s) instances, error info is %s", cli.target.GetID(), rsp.ErrMsg)
		return false, cli.params.Err.Error(common.CCErrTopoInstSelectFailed)
	}

	return 0 != rsp.Data.Count, nil
}
func (cli *inst) Save() error {

	if exists, err := cli.IsExists(); nil != err {
		return err
	} else if exists {
		return cli.Update()
	}

	return cli.Create()
}

func (cli *inst) GetObject() model.Object {
	return cli.target
}

func (cli *inst) GetInstID() (int64, error) {
	return cli.datas.Int64(cli.target.GetInstIDFieldName())
}
func (cli *inst) GetParentID() (int64, error) {
	return cli.datas.Int64(common.BKInstParentStr)
}

func (cli *inst) GetInstName() (string, error) {

	return cli.datas.String(cli.target.GetInstNameFieldName())
}

func (cli *inst) ToMapStr() frtypes.MapStr {
	return cli.datas
}
func (cli *inst) SetValue(key string, value interface{}) error {
	cli.datas.Set(key, value)
	return nil
}

func (cli *inst) SetValues(values frtypes.MapStr) {
	cli.datas.Merge(values)
}

func (cli *inst) GetValues() frtypes.MapStr {
	return cli.datas
}